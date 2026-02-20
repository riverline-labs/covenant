# Covenant POC Design

**Date:** 2026-02-19
**Status:** Approved

## Overview

A proof-of-concept implementation of the Covenant v0.3.0 specification. Three components in one Go module (`covenant-poc`): a contract server, a generic executor, and a CLI client. The billing domain is the example domain.

## Architecture

```
contract-server (:8081)    executor (:8080)       cli
  GET /.well-known/covenant  POST /execute     ──► execute
  GET /contracts/**     ◄── fetch CUE at boot       │
        │                        │                   │
   contracts/ dir           CUE Go SDK        ◄──────┘
   (local filesystem)       parse + evaluate
                                 │
                           port adapters
                           (in-memory)
```

### Components

**contract-server** — Thin HTTP file server. Serves `.cue` files from a configurable local directory. Exposes `GET /.well-known/covenant` (discovery doc with ETag, service name, contract paths) and `GET /contracts/**` (raw CUE file content). No business logic.

**executor** — Generic execution engine. On startup, fetches the discovery doc, fetches all domain CUE files, compiles them with `cuelang.org/go/cue`, walks the value tree to populate Go structs. Evaluates operations per Section 11 of the Covenant spec. Polls for ETag changes every 30 seconds.

**cli** — Command-line client. Fetches discovery, builds an execute request, prints the response.

## CUE Contract Files

Location: `contracts/` (filesystem directory, not embedded). Structured for machine readability by the CUE Go SDK.

```
contracts/
├── common/
│   ├── money.cue
│   └── error.cue
└── billing/
    ├── facts.cue         # base facts + derived facts
    ├── entities.cue      # invoice state machine
    ├── rules.cue         # business rules + verdicts
    ├── operations.cue    # operation contracts
    └── flows.cue         # persona flows
```

Facts are structured with explicit `source`, `required`, `on_missing` fields. Derived facts include `derivation.fn` and `derivation.args`. Rules include `applies_to`, `when` conditions, and `verdict` structs. All designed so `LookupPath` + `Fields()` / `List()` on a compiled `cue.Value` yields clean Go extraction.

## Executor: Contract Loading

1. `GET /.well-known/covenant` → parse ETag and contract paths.
2. Fetch each `.cue` file (facts, entities, rules, operations, flows, common types).
3. `cuecontext.New()` + `ctx.CompileBytes()` for each file; unify into one `cue.Value`.
4. Walk value tree:
   - `val.LookupPath("facts").Fields()` → `[]FactDef`
   - `val.LookupPath("derived_facts").Fields()` → `[]DerivedFactDef`
   - `val.LookupPath("rules").List()` → `[]RuleDef`
   - `val.LookupPath("operations").Fields()` → `[]OperationDef`
   - `val.LookupPath("entities").Fields()` → `[]EntityDef`
   - Complex nested structs (conditions, derivations) extracted via sub-`cue.Value` marshalled to JSON → Go structs.
5. Store compiled `Contract` struct in engine; update ETag.

## Executor: Evaluation Algorithm (Section 11)

All steps 1–5 are side-effect-free.

1. **GATHER** base facts: input facts from request body; context facts (user roles, etc.); port facts fetched in parallel goroutines. `on_missing` policy applied per fact on port failure. `FactSet` protected by a mutex.
2. **DERIVE** computed facts: topological sort of derived fact DAG; evaluate each using the stdlib (comparison, arithmetic, boolean).
3. **VALIDATE** entity state: check operation's required entity state matches current; check transition is valid.
4. **EVALUATE** rules: select rules where `applies_to` includes this operation; evaluate conditions against fact set; collect verdicts.
5. **APPLY** verdict: `deny` → return error envelope; `escalate` → queue; `require` → return conditions; `flag` → attach warning and proceed; no verdict → proceed.
6. **EXECUTE**: invoke port adapter `Execute`. Side effects here only.
7. **TRANSITION**: update entity state.

Dry-run stops after step 5 and returns the resolved verdict + fact snapshot.

## Port Adapters (In-Memory)

All ports are injected via a `PortRegistry`. They do not enforce policy.

| Port | Impl | Seeded data |
|---|---|---|
| `customerRepo` | `inmem.CustomerRepo` | cust_123 (active), cust_456 (closed), cust_789 (suspended) |
| `invoiceRepo` | `inmem.InvoiceRepo` | inv_001 ($1500, approved, cust_123), inv_002 ($250, draft, cust_123), inv_003 ($25000, approved, cust_456) |
| `paymentProcessor` | `inmem.PaymentProcessor` | status: up; `SetStatus("down")` for simulation |

## CLI Usage

```
go run ./cli --op GetInvoice --invoice inv_001
go run ./cli --op ProcessPayment --invoice inv_001 --amount 500 --dry-run
go run ./cli --op ProcessPayment --invoice inv_001 --amount 500
go run ./cli --op ProcessPayment --customer cust_456 --invoice inv_003 --amount 100   # denied
go run ./cli --op ProcessPayment --invoice inv_001 --amount 5000   # exceeds balance
```

## Key Cleanups vs `start` File

- `FactSet` concurrent writes: add `sync.Mutex`
- `randString`: use `math/rand` with a proper seed
- Dotted-path resolution for nested facts: `payment.amount.value` → get `payment.amount` (a map), look up `value`
- Port lookup: strip `port:` prefix from `source` field
- `ParallelGroups` topological sort: fix node removal during in-degree tracking
- Input fact key mapping: use dotted names consistently (`customer.id` not `customer_id`) in the fact namespace
