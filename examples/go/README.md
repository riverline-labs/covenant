# Covenant POC

A proof-of-concept implementation of the [Covenant v0.3.0 specification](../COVENANT.md) in Go.

## Architecture

```
contract-server (:26861)    executor (:26860)         cli
  GET /.well-known/covenant   POST /execute        ──► sends requests
  GET /contracts/**      ◄── fetches CUE at boot        │
        │                         │                      │
  contracts/ directory       CUE Go SDK            ◄─────┘
  (local filesystem)         compile + evaluate
                                  │
                            port adapters
                            (in-memory)
```

Three components, one Go module:

- **contract-server** — Thin HTTP file server. Serves `.cue` files from a local directory. Exposes `/.well-known/covenant` (discovery) and `/contracts/**` (raw CUE).
- **executor** — Generic evaluation engine. Fetches CUE files from the contract server, compiles them with `cuelang.org/go/cue`, extracts the contract definition, and evaluates operations per Section 11 of the Covenant spec.
- **cli** — Command-line client.

## Running

**Terminal 1 — start the contract server:**
```bash
cd examples/go
go run ./contract-server --dir ./contracts
# listening on :26861
```

**Terminal 2 — start the executor:**
```bash
go run ./executor --contracts http://localhost:26861
# listening on :26860
```

**Terminal 3 — use the CLI:**
```bash
# Get invoice details
go run ./cli --op GetInvoice --invoice inv_001

# Dry run a payment (evaluate rules only, no side effects)
go run ./cli --op ProcessPayment --invoice inv_001 --amount 500 --dry-run

# Process a payment
go run ./cli --op ProcessPayment --invoice inv_001 --amount 500

# Denied: payment exceeds invoice balance
go run ./cli --op ProcessPayment --invoice inv_001 --amount 5000

# Denied: closed account
go run ./cli --op ProcessPayment --customer cust_456 --invoice inv_003 --amount 100

# Flagged: large payment (dry run shows deny + flag verdicts)
go run ./cli --op ProcessPayment --invoice inv_001 --amount 15000 --dry-run
```

## Seeded Data

| ID | Type | Details |
|---|---|---|
| `cust_123` | Customer | active |
| `cust_456` | Customer | closed |
| `cust_789` | Customer | suspended |
| `inv_001` | Invoice | approved, $1500, owned by cust_123 |
| `inv_002` | Invoice | draft, $250, owned by cust_123 |
| `inv_003` | Invoice | approved, $25000, owned by cust_456 |

## Key Design Decisions

**CUE at runtime:** The executor fetches raw `.cue` files and uses the CUE Go SDK (`cuelang.org/go/cue`) to compile and walk the value tree. No code generation — contracts are the source of truth.

**Section 11 evaluation order:** Gather facts → Derive computed facts → Evaluate rules → Apply verdict → Execute (side effects here only). Steps 1–4 are side-effect-free.

**Fact resolution:** Dotted paths like `payment.amount.value` resolve to the base fact `payment.amount` (a nested map) and navigate into its `value` field.

**Port adapters:** `customerRepo`, `invoiceRepo`, and `paymentProcessor` are in-memory. They retrieve facts and execute operations — no policy logic.

## Not Yet Implemented

on_missing port failure handling — The engine supports on_missing on fact definitions but port failure paths (timeout, unavailable, null) are unexercised and untested. Section 14.5 requires that FACT_UNAVAILABLE be distinguishable from business-rule denials in both the response and audit record.

Audit records — The executor logs operation outcomes to stdout but does not produce structured audit records as required by Section 14.6. The required fields (invocation_id, fact_snapshot, rules_matched, contract_version, etc.) are not persisted.

Fact path resolution is unspecified — The executor resolves dotted fact paths like payment.amount.value by treating payment.amount as the base fact and navigating into its value field. This behavior is not defined in the spec. Section 4 assumes 
facts are scalars; the spec will need to either formalize the dotted-path traversal convention or require that all facts are scalar values.

Discovery response shape diverges from spec — Section 13.2 specifies "cue": "/contracts/billing/" as a directory reference. This POC serves an enumerated file list instead ("files": [...]). The enumerated approach is more useful for agents and may prompt a spec update, but currently represents a divergence.

## What This Tells Us

This POC has done exactly what a POC should do: revealed gaps between theory and practice. The gaps fall into three categories:
- Implementation gaps — things the spec requires that this POC hasn't built yet: on_missing failure handling and structured audit records.
- Spec gaps — behaviors the implementation had to invent because the spec doesn't define them: dotted fact path resolution. These need spec clarification to ensure interoperability across executor implementations.
- Spec improvements — places where the implementation found a better answer than the spec currently describes: the enumerated file list in discovery is more useful than a directory reference and may warrant a spec update.