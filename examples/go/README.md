# Covenant POC

A proof-of-concept implementation of the [Covenant v0.3.0 specification](../COVENANT.md) in Go.

## Architecture

```
contract-server (:26861)    executor (:26860)         cli
  GET /.well-known/covenant   POST /execute        ──► sends requests
  GET /contracts/**      ◄── fetches CUE at boot         │
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
