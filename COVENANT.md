---
title: "Covenant"
subtitle: "Contract-First Systems for Human-AI Collaboration"
version: "0.2.0"
date: 2026-02-19
contributors:
  - name: Brandon
    role: Author
  - name: Claude (Anthropic)
    role: Contributor
  - name: DeepSeek
    role: Contributor
  - name: GPT (OpenAI)
    role: Contributor
  - name: Grok (xAI)
    role: Contributor
  - name: Gemini (Google)
    role: Contributor
license: TBD
repository: TBD
---

# Covenant

**Contract-First Systems for Human-AI Collaboration**

A specification for building systems where humans and AI agents collaborate through machine-readable contracts. The contracts are the source of truth; implementation is disposable. Agents reason about what they can do by reading contracts, not code.

---

## 1. Core Principles

### 1.1 Contract-First, Not Code-First

The contract is authoritative. All system behavior is expressed in declarative contracts before any code is written. Implementation is derived from contracts, never the reverse.

### 1.2 Regeneration-Safe

Contracts are the source of truth. Implementation can be fully regenerated at any time. No hand-edited code lives in the contract layer. Business logic must never migrate into adapters or generated code. If behavior cannot be expressed in the contract, the contract language is insufficient — not the implementation.

### 1.3 Agent-Readable by Design

AI agents are first-class citizens. Contracts are structured for machine consumption first, human readability second. An agent should be able to determine — without reading code — what operations exist, what rules apply, what paths are available, and how to handle failure.

### 1.4 Human-AI Covenant

The name expresses the mutual obligation: Humans maintain accurate, complete contracts. AI agents operate faithfully within them. Neither can do their job without the other.

---

## 2. Authoritative Format: CUE

All contracts are written in CUE, a declarative configuration language chosen for:

- **Unification of schema and data** — no parallel validation
- **Built-in constraints** — fact definitions are the type system
- **Non-Turing completeness by design** — the contract layer is decidable
- **First-class imports** — cross-domain sharing without duplication
- **Export to JSON, YAML, OpenAPI** — for consumers who need other formats

CUE is the single source of truth. Everything else (OpenAPI specs, JSON Schemas, generated code, documentation) is derived.

**Portability note:** The Covenant philosophy requires that the contract layer be non-Turing complete and support schema/data unification. CUE is the reference implementation. Future implementations may use other languages that satisfy these constraints.

---

## 3. Contract Dependency Graph

Within a domain, contracts have a strict dependency direction. This acyclicity is the backbone of the entire system.

```
┌─────────────────┐
│   Common Types  │  (money.cue, customer.cue, error.cue)
└────────┬────────┘
         │ imports
         ▼
┌─────────────────┐
│   Domain Facts  │  (billing/facts.cue)
└────────┬────────┘
         │ referenced by
         ▼
┌─────────────────┐
│  Entity States  │  (billing/entities.cue)
└────────┬────────┘
         │ constrained by
         ▼
┌─────────────────┐
│   Domain Rules  │  (billing/rules.cue)
└────────┬────────┘
         │ applied to
         ▼
┌─────────────────┐
│   Operations    │  (billing/operations.cue)
└────────┬────────┘
         │ composed into
         ▼
┌─────────────────┐
│      Flows      │  (billing/flows.cue)
└─────────────────┘
```

**Dependency Rules:**

- Flows compose Operations and reference their outputs
- Operations declare inputs, outputs, errors, and which Rules constrain them
- Rules reference Facts and Entity States in conditions; use Common Types for type definitions
- Entity States reference Facts for transition guards
- Facts reference Common Types and declare Ports as sources
- Common Types have no internal dependencies
- **No reverse dependencies.** A Rule cannot define a Fact. A Fact cannot reference a Flow. An Operation cannot define a Rule. Each layer can be understood, validated, and generated independently.

**Cross-domain dependencies** are handled via CUE imports of Common Types only. Billing Rules cannot directly reference Shipping Facts. If a cross-domain fact is needed, it must be lifted to Common Types first — an explicit signal that domains are less separate than originally thought. See Section 3.2 for governance of Common Types.

### 3.1 Repository Structure

```
contracts/
├── cue.mod/                  # dependencies
├── common/                   # shared types across domains
│   ├── money.cue
│   ├── customer.cue
│   ├── error.cue
│   ├── personas.cue
│   └── stdlib.cue            # derivation function standard library
├── billing/                  # billing domain
│   ├── facts.cue            # domain facts with types
│   ├── entities.cue         # entity state machines
│   ├── rules.cue            # business rules
│   ├── operations.cue       # operation contracts
│   └── flows.cue            # persona paths
├── shipping/                 # shipping domain
│   ├── facts.cue
│   ├── entities.cue
│   ├── rules.cue
│   ├── operations.cue
│   └── flows.cue
└── generated/                # derived views (not source of truth)
    ├── billing.openapi.json
    └── shipping.openapi.json
```

### 3.2 Governance of Common Types

Common Types are shared infrastructure. Without governance, `common/` becomes a junk drawer that collapses domain boundaries.

**Rules for Common Types:**

- A type may only be lifted to `common/` when two or more domains require it
- Every type in `common/` must have a clear owner (a domain or "platform")
- Lifting requires the same consensus process as domain contracts (Section 12)
- `common/` is reviewed on a regular cadence; types that are only used by one domain are demoted back
- The derivation standard library (`stdlib.cue`) follows its own versioning (see Section 4.3)

---

## 4. Facts

Facts are named, typed values the system knows about. Each fact declares its source and shape.

### 4.1 Fact Definition

```cue
// billing/facts.cue
package billing

import "common/money"

facts: {
  // From request input
  "payment.amount": money.Amount & {
    source: "input"
  }

  // From context (authentication, tenant, etc.)
  "user.roles": [...string] & {
    source: "ctx"
  }

  // From repositories via ports
  "customer.status": "active" | "suspended" | "closed" & {
    source: "port:customerRepo"
  }

  "invoice.balance": money.Amount & {
    source: "port:invoiceRepo"
  }

  "invoice.due_date": time.Time & {
    source: "port:invoiceRepo"
  }

  // Historical materializations present as scalars
  "customer.failed_payment_count": int & {
    source:      "port:customerRepo"
    description: "Count of failed payments in rolling 90-day window"
  }
}
```

**Historical and aggregate facts:** Real business systems require temporal invariants and historical aggregation ("three strikes" logic, rolling windows, etc.). These are materialized by Ports and presented as scalar facts. The contract declares what the fact is and where it comes from; the Port is responsible for computing it. This keeps the fact layer simple while acknowledging that the data behind a fact may involve complex queries.

### 4.2 Derived Facts

Some values are deterministic functions of other facts. Derived facts make these explicit without introducing arbitrary computation.

```cue
// billing/facts.cue (continued)

derived_facts: {
  "invoice.is_overdue": bool & {
    derivation: {
      fn:   "date_before"
      args: [{ fact: "invoice.due_date" }, { value: "now" }]
    }
  }

  "payment.exceeds_balance": bool & {
    derivation: {
      fn:   "greater_than"
      args: [{ fact: "payment.amount" }, { fact: "invoice.balance" }]
    }
  }

  "customer.is_high_risk": bool & {
    derivation: {
      fn:   "greater_or_equal"
      args: [{ fact: "customer.failed_payment_count" }, { value: 3 }]
    }
  }
}
```

**Derivation constraints:**

- Derivation functions are drawn from the **Derivation Standard Library** (see Section 4.3)
- Derived facts may reference base facts or other derived facts, but must form a DAG (no cycles)
- Derived facts are evaluated before rules execute and are then treated as ordinary facts
- An agent or tool can statically verify that derived facts terminate and produce bounded results

### 4.3 Derivation Standard Library

The closed set of derivation functions is defined in `common/stdlib.cue` and is versioned independently from domain contracts. This prevents the function set from becoming a backdoor for Turing creep.

```cue
// common/stdlib.cue
package common

stdlib_version: "0.1.0"

functions: {
  // Comparison
  "equals":           { args: 2, returns: "bool" }
  "not_equals":       { args: 2, returns: "bool" }
  "greater_than":     { args: 2, returns: "bool" }
  "greater_or_equal": { args: 2, returns: "bool" }
  "less_than":        { args: 2, returns: "bool" }
  "less_or_equal":    { args: 2, returns: "bool" }

  // Arithmetic
  "add":              { args: 2, returns: "number" }
  "subtract":         { args: 2, returns: "number" }
  "multiply":         { args: 2, returns: "number" }
  "divide":           { args: 2, returns: "number" }
  "modulo":           { args: 2, returns: "number" }

  // Date/Time
  "date_before":      { args: 2, returns: "bool" }
  "date_after":       { args: 2, returns: "bool" }
  "date_diff_days":   { args: 2, returns: "number" }

  // String
  "contains":         { args: 2, returns: "bool" }
  "starts_with":      { args: 2, returns: "bool" }
  "ends_with":        { args: 2, returns: "bool" }
  "to_lower":         { args: 1, returns: "string" }
  "to_upper":         { args: 1, returns: "string" }

  // Collection
  "in":               { args: 2, returns: "bool", description: "Value is member of list" }
  "length":           { args: 1, returns: "number" }
  "is_empty":         { args: 1, returns: "bool" }

  // Logical
  "and":              { args: 2, returns: "bool" }
  "or":               { args: 2, returns: "bool" }
  "not":              { args: 1, returns: "bool" }
}
```

**Standard library governance:**

- Additions to the standard library go through the debate protocol (Section 12)
- Every function must be pure, total, and terminating
- Functions must not perform I/O, access ports, or reference external state
- The standard library is versioned; contracts declare which version they depend on
- A function may be deprecated but never removed while any active contract references it

### 4.4 Fact Principles

- **Immutable** for the duration of a single evaluation
- **Closed-world** — no facts can be added during evaluation
- **Typed** — every fact has a CUE type that constrains its values
- **Sourced** — every base fact declares where it comes from
- **Deterministic** — derived facts always produce the same output for the same inputs
- **Scalar presentation** — historical and aggregate facts are materialized by Ports and presented as simple values

---

## 5. Entity State Machines

Entities have states. Transitions between states are declared, not implied. This makes the valid lifecycle of every entity explicit and statically verifiable.

### 5.1 Entity Definition

```cue
// billing/entities.cue
package billing

entities: {
  "invoice": {
    states: ["draft", "submitted", "approved", "pending_review", "paid", "rejected", "cancelled"]

    initial: "draft"

    terminal: ["paid", "rejected", "cancelled"]

    transitions: [
      { from: "draft",          to: "submitted",      via: "SubmitInvoice" },
      { from: "submitted",      to: "approved",        via: "ApproveInvoice" },
      { from: "submitted",      to: "pending_review",  via: "EscalateInvoice" },
      { from: "submitted",      to: "rejected",        via: "RejectInvoice" },
      { from: "pending_review", to: "approved",        via: "ApproveInvoice" },
      { from: "pending_review", to: "rejected",        via: "RejectInvoice" },
      { from: "approved",       to: "paid",            via: "ProcessPayment" },
      { from: "*",              to: "cancelled",       via: "CancelInvoice",
        guard: { not_in: ["paid", "cancelled"] }
      },
    ]
  }
}
```

### 5.2 Entity State Principles

- **Exhaustive** — every valid state is declared; undeclared states are invalid
- **Explicit transitions** — every valid transition names its source state, target state, and the operation that triggers it
- **Guards** — transitions may have conditions expressed as fact references
- **Terminal states** — states from which no further transitions are possible
- **Static verification** — tooling can verify that flows only attempt valid transitions, that all states are reachable, and that terminal states are reachable from all non-terminal states
- **Visualization** — entity definitions can be rendered directly as state machine diagrams

---

## 6. Rules

Rules encode business policy. They are declarative, non-Turing complete, and produce verdicts.

### 6.1 Rule Structure

```cue
// billing/rules.cue
package billing

import "common/error"

rules: [
  {
    id:         "block-payment-closed-account"
    applies_to: ["ProcessPayment"]

    when: {
      all: [
        { fact: "customer.status", equals: "closed" }
      ]
    }

    verdict: deny: {
      code:   "ACCOUNT_CLOSED"
      reason: "Cannot process payments for closed accounts"

      error: error.Envelope & {
        httpStatus: 422
        category:   "business_rule_violation"
        retryable:  false
        suggestion: "Reactivate account or use different payment method"
      }
    }
  },

  {
    id:         "large-refund-review"
    applies_to: ["IssueRefund"]

    when: {
      all: [
        { fact: "refund.amount", greater_than: 10000 }
      ]
    }

    verdict: escalate: {
      queue:  "finance-review"
      reason: "Refunds over 10,000 require human approval"
    }
  }
]
```

### 6.2 Verdict Types

| Verdict    | Meaning                                                         |
|------------|-----------------------------------------------------------------|
| `deny`     | Operation cannot proceed. Returns error envelope.               |
| `escalate` | Requires human intervention. May be queued.                     |
| `require`  | Additional conditions must be satisfied before proceeding.      |
| `flag`     | Warning only. Does not block execution.                         |

**No explicit `allow`.** The absence of a `deny`, `escalate`, or `require` verdict is permission. This eliminates ambiguity about what `allow` means and removes the need for priority reasoning between allow and other verdicts.

### 6.3 Conflict Resolution

When multiple rules match, verdicts are resolved by priority:

```
deny > escalate > require > flag
```

The highest-priority verdict wins. When multiple rules produce the same verdict type, all are returned (e.g., two `flag` verdicts both surface; two `require` verdicts both must be satisfied).

**Interaction between verdict types:** Verdicts are independent. A `require` cannot satisfy a `deny`. If one rule denies and another requires, the deny wins and the operation does not proceed. There is no mechanism for rules to interact with or override each other. This is a deliberate limitation that prevents rule-to-rule coupling.

### 6.4 Rule Principles

- **Non-Turing complete** — conditions use only `all`, `any`, `not` combinators with operators from the standard library
- **No computed facts** — rules cannot define new facts
- **No rule-to-rule dependencies** — rules are independent; evaluation order does not matter
- **No mutation** — rules observe, they do not change state
- **Side-effect-free** — rule evaluation must never trigger side effects. Deny and escalate verdicts must never cause operations to partially execute.

---

## 7. Operations

Operations are the verbs of the system. Each operation declares its full contract: what it accepts, what it returns, what can go wrong, and what constrains it.

### 7.1 Operation Definition

```cue
// billing/operations.cue
package billing

import (
  "common/money"
  "common/error"
)

operations: {
  "CreateInvoice": {
    input: {
      customer_id: string
      line_items:  [...{
        description: string
        amount:      money.Amount
      }]
    }

    output: {
      invoice_id: string
      status:     "draft"
      total:      money.Amount
    }

    errors: [
      error.Envelope & { code: "CUSTOMER_NOT_FOUND",  httpStatus: 404, category: "validation", retryable: false },
      error.Envelope & { code: "INVALID_LINE_ITEMS",   httpStatus: 422, category: "validation", retryable: false },
    ]

    constrained_by: []

    transitions: [
      { entity: "invoice", to: "draft" }
    ]
  }

  "ProcessPayment": {
    input: {
      invoice_id:     string
      payment_method: "card" | "bank_transfer" | "credit"
      amount:         money.Amount
    }

    output: {
      payment_id:  string
      status:      "completed" | "pending"
      receipt_url: string
    }

    errors: [
      error.Envelope & {
        code:       "ACCOUNT_CLOSED"
        httpStatus: 422
        category:   "business_rule_violation"
        retryable:  false
        suggestion: "Reactivate account or use different payment method"
      },
      error.Envelope & {
        code:        "INSUFFICIENT_FUNDS"
        httpStatus:  402
        category:    "business_rule_violation"
        retryable:   true
        retry_after: "PT30M"
        suggestion:  "Add funds to account or reduce payment amount"
      },
      error.Envelope & {
        code:                "PAYMENT_PROVIDER_UNAVAILABLE"
        httpStatus:          503
        category:            "external_dependency"
        retryable:           true
        retry_after:         "PT5M"
        fallback_operations: ["ProcessPaymentAlternate"]
      },
    ]

    constrained_by: [
      "block-payment-closed-account",
      "block-payment-insufficient-funds",
    ]

    transitions: [
      { entity: "invoice", from: "approved", to: "paid" }
    ]
  }

  "IssueRefund": {
    input: {
      order_id: string
      amount:   money.Amount
      reason:   string
    }

    output: {
      refund_id: string
      status:    "processed" | "pending_review"
    }

    errors: [
      error.Envelope & { code: "REFUND_LIMIT_EXCEEDED", httpStatus: 422, category: "business_rule_violation", retryable: false },
      error.Envelope & { code: "ORDER_TOO_OLD",         httpStatus: 422, category: "business_rule_violation", retryable: false },
      error.Envelope & { code: "DUPLICATE_REFUND",       httpStatus: 409, category: "validation",               retryable: false },
    ]

    constrained_by: [
      "large-refund-review",
    ]

    transitions: []
  }
}
```

### 7.2 Operation Principles

- **Self-describing** — an agent can read one operation and know its full interface without consulting other files
- **Input/output typed** — CUE types constrain what goes in and what comes out
- **Errors enumerated** — every known failure mode is declared with its error envelope
- **Constraints linked** — the operation names which rules apply to it (cross-referenced with `applies_to` in rules)
- **Transitions declared** — the operation names which entity state transitions it triggers
- **Effectful** — operations may have side effects via ports. Rule evaluation must complete before any side effects execute.

---

## 8. Personas and Authorization

Personas represent identities that can perform operations. They are first-class in the contract.

### 8.1 Authority Model

**Personas are the single source of authorization truth.** A persona's `can` list is the authoritative declaration of what operations that persona may invoke. Operations do not independently declare who can use them.

Operations may declare **invocation conditions** under which certain personas require additional steps (approval, MFA, etc.), but the question of "who can call this at all" is answered exclusively by personas.

**Validation rule:** Tooling must verify that every operation referenced in any flow is present in the `can` list of the flow's persona (or in the `can` list of a persona explicitly named via `as:`). An operation that no persona can invoke is dead code.

### 8.2 Persona Definition

```cue
// common/personas.cue
package common

personas: {
  "billing_admin": {
    description: "Full access to billing operations"
    can: [
      "CreateInvoice",
      "ApproveInvoice",
      "ProcessPayment",
    ]
  }

  "support_agent": {
    description: "Customer-facing support with limited billing access"
    can: [
      "LookupOrder",
      "VerifyCustomerIdentity",
      "IssueRefund",
    ]
  }

  "refund_approver": {
    description: "Approves refunds escalated from support"
    can: ["ApproveRefund"]
    requires_mfa: true
  }

  "finance": {
    description: "Back-office financial operations"
    can: ["ProcessRefund"]
  }
}
```

### 8.3 Operation-Level Conditions

When an operation requires additional steps for certain personas, this is expressed as an **invocation condition** on the operation:

```cue
// billing/operations.cue (partial)
operations: {
  "IssueRefund": {
    // ... input, output, errors, etc.

    invocation_conditions: {
      "support_agent": {
        requires:      "approval"
        approval_from: "refund_approver"
        reason:        "Support agents require approval for all refunds"
      }
    }
  }
}
```

This is not an authorization declaration — the persona already has `IssueRefund` in its `can` list. It is a constraint on *how* the operation is invoked by that persona.

### 8.4 Flow Persona Boundaries

Flows have a primary persona but may cross boundaries explicitly:

```cue
flows: [
  {
    id:      "support-refund"
    persona: "support_agent"
    goal:    "Process a customer refund from support"

    steps: [
      { operation: "LookupOrder" },
      { operation: "VerifyCustomerIdentity" },
      {
        operation:    "ApproveRefund"
        as:           "refund_approver"
        reason:       "Refunds require approval from a different persona"
        requires_mfa: true
      },
      {
        operation: "ProcessRefund"
        as:        "finance"
        reason:    "Financial processing requires finance persona"
      }
    ]
  }
]
```

When a step requires a different persona:

- The agent may request **temporary elevation** (with audit trail)
- The flow may **hand off** to a different agent
- The system may **reject** if elevation isn't possible

Every persona boundary crossing is explicit and auditable.

### 8.5 Elevation Protocol

Temporary persona elevation is a sensitive operation. The contract declares the *possibility* of elevation; the runtime governs execution.

```cue
// Runtime elevation request (not stored in contracts)
elevation_request: {
  flow_id:          "support-refund"
  step:             3
  requesting_agent: "agent-1234"
  from_persona:     "support_agent"
  to_persona:       "refund_approver"
  reason:           "Refunds require approval from a different persona"
  requires_mfa:     true
  timestamp:        "2026-02-19T10:05:00Z"
}

elevation_grant: {
  request_id:  "elev-5678"
  granted_by:  "human-operator" | "system-policy"
  duration:    "PT15M"
  scope:       ["ApproveRefund"]
  audit_trail: true
  revocable:   true
}
```

**Elevation principles:**

- Elevation is scoped to specific operations, not full persona assumption
- Elevation has a declared duration after which it expires
- Elevation is auditable and revocable
- The contract declares what elevations are possible; the runtime decides whether to grant them

---

## 9. Flows and Snapshots

Flows are sequences of operations that accomplish a goal. They are the unit of agent work and the unit of versioning.

### 9.1 Flow Definition

```cue
// billing/flows.cue
package billing

flows: [
  {
    id:      "invoice-to-payment"
    persona: "billing_admin"
    goal:    "Move invoice from draft to paid"

    steps: [
      {
        operation: "CreateInvoice"
        produces:  { entity: "invoice", state: "draft" }
      },
      {
        operation: "SubmitInvoice"
        requires:  { entity: "invoice", state: "draft" }
        produces:  { entity: "invoice", state: "submitted" }
      },
      {
        operation: "ApproveInvoice"
        requires:  { entity: "invoice", state: "submitted" }
        produces:  { entity: "invoice", state: "approved" }
      },
      {
        operation: "ProcessPayment"
        requires:  { entity: "invoice", state: "approved" }
        produces:  { entity: "invoice", state: "paid" }
      }
    ]
  }
]
```

**Static validation:** Tooling must verify that every `requires`/`produces` pair in a flow corresponds to a valid transition in the entity state machine (Section 5). A flow that attempts an invalid transition is a contract error.

### 9.2 Branching in Flows

Flows may include conditional branches, but branching is constrained to **verdict-driven routing**. Flows do not have their own condition language.

```cue
{
  id:      "invoice-lifecycle"
  persona: "billing_admin"
  goal:    "Process invoice through to payment or rejection"

  steps: [
    {
      operation: "CreateInvoice"
      produces:  { entity: "invoice", state: "draft" }
    },
    {
      operation: "SubmitInvoice"
      produces:  { entity: "invoice", state: "submitted" }
    },
    {
      operation: "ReviewInvoice"
      on_verdict: {
        "approve": {
          operation: "ApproveInvoice"
          produces:  { entity: "invoice", state: "approved" }
        }
        "escalate": {
          operation: "EscalateInvoice"
          produces:  { entity: "invoice", state: "pending_review" }
        }
      }
    }
  ]
}
```

**Branching constraints:**

- Branch keys correspond to verdict outcomes from the rule engine (`on_verdict`)
- This keeps all conditional logic in the Rules layer where it belongs
- Flows remain pure composition of operations — they describe *sequence*, not *logic*
- Every branch must produce a valid entity state transition

### 9.3 Flow Snapshots

When an agent begins a flow, it receives a snapshot of the relevant rules as they existed at that moment. The snapshot lives for the duration of the flow.

```cue
// Runtime snapshot (not stored in contracts)
flow_instance: {
  flow_id:        "invoice-to-payment"
  correlation_id: "corr-abc-123"
  snapshot_time:  "2026-02-19T10:00:00Z"
  stdlib_version: "0.1.0"
  rules:          [ /* rules as of snapshot_time */ ]
  current_step:   2
  state:          { invoice: { id: "INV-001", status: "submitted" } }
}
```

**Benefits:**

- Agents operate in a consistent logical universe mid-flow
- Rules can evolve without breaking in-progress work
- Snapshots are discardable when flows complete
- No global rule versioning needed

### 9.4 Snapshot Lifecycle

Snapshots are not permanent. They have explicit lifecycle semantics.

**Creation:** A snapshot is created when an agent begins a flow. It captures the rules, derived fact definitions, standard library version, and entity state definitions as of that moment.

**Correlation:** Every snapshot has a `correlation_id` that links all operations and events within the flow instance. If the system restarts, the snapshot can be resumed using this ID.

**Expiration:** Snapshots have a maximum time-to-live (TTL), declared per flow:

```cue
flows: [
  {
    id:           "invoice-to-payment"
    snapshot_ttl: "P7D"
    // ...
  },
  {
    id:           "refund-appeal"
    snapshot_ttl: "P90D"
    // ...
  }
]
```

**On expiration:**

- The system notifies the agent that the snapshot has expired
- The agent may request a **snapshot refresh**: a new snapshot is created from current rules, and the flow resumes from its current step under the new rules
- Alternatively, the flow may be aborted
- The choice between refresh and abort is declared per flow as `on_expiry: "refresh" | "abort" | "human_review"`

**Garbage collection:** Completed flow snapshots are garbage-collected immediately. Expired snapshots follow the declared `on_expiry` policy.

---

## 10. Error Contract

Every operation can fail. The failure modes are part of the contract, so agents can plan for them.

### 10.1 Error Envelope Definition

```cue
// common/error.cue
package common

Envelope: {
  code:        string
  message:     string
  httpStatus:  int
  category:    "validation" | "business_rule_violation" |
               "authorization" | "system" | "external_dependency"
  retryable:   bool

  // Retry guidance (present when retryable is true)
  retry_after?: string   // ISO 8601 duration, e.g. "PT5M"

  // Recovery guidance
  suggestion?:              string
  fallback_operations?:     [...string]
  human_escalation_fields?: [...string]

  details?: {...}
}
```

### 10.2 Rule Error Specifications

Each rule that can deny includes an error envelope:

```cue
{
  id:         "insufficient-funds"
  applies_to: ["ProcessPayment"]

  when: {
    all: [
      { fact: "payment.exceeds_balance", equals: true }
    ]
  }

  verdict: deny: {
    code:   "INSUFFICIENT_FUNDS"
    reason: "Account balance insufficient"

    error: common.Envelope & {
      httpStatus:  402
      category:    "business_rule_violation"
      retryable:   true
      retry_after: "PT30M"
      suggestion:  "Add funds to account or reduce payment amount"
    }
  }
}
```

Agents plan for failure paths — retry logic, fallback flows, human escalation — based on the error contract, not trial and error.

---

## 11. Evaluation Algorithm

When an operation is invoked, the system follows a strict evaluation sequence. This is **normative**.

### 11.1 Evaluation Steps

```
1. GATHER base facts
   ├── Collect facts from input (source: "input")
   ├── Collect facts from context (source: "ctx")
   └── Collect facts from ports (source: "port:*")

2. DERIVE computed facts
   ├── Topologically sort derived facts by dependency
   ├── Evaluate each derived fact using stdlib functions
   └── Add derived facts to the fact set

3. VALIDATE entity state
   ├── Check that the operation's required entity state matches current state
   ├── Check that the target transition is valid per entity state machine
   └── If invalid → return error, do not proceed

4. EVALUATE rules
   ├── Select rules where applies_to includes this operation
   ├── Evaluate each rule's conditions against the fact set
   ├── Collect all matching verdicts
   └── Resolve conflicts by priority: deny > escalate > require > flag

5. APPLY verdict
   ├── If deny    → return error envelope, no side effects
   ├── If escalate → queue for human review, no side effects
   ├── If require  → return required conditions to agent, no side effects
   ├── If flag     → attach warnings, proceed to step 6
   └── If no verdict → proceed to step 6

6. EXECUTE operation
   ├── Invoke operation logic via ports
   ├── Side effects occur here and only here
   └── Return output

7. TRANSITION entity state
   ├── Update entity state per declared transition
   └── Record state change

8. ADVANCE flow
   ├── Update flow instance current_step
   ├── If on_verdict branching → select branch
   └── If final step → complete flow, garbage-collect snapshot
```

**Critical invariant:** Steps 1–5 are **side-effect-free**. No ports are written to, no external systems are called, no state is mutated. Side effects occur only in step 6, and only if the verdict permits execution. This ensures that deny and escalate verdicts are always safe.

---

## 12. Design Process: Exhaustion as Fitness Filter

Before a domain is implemented, AI personas debate its contracts until either consensus or exhaustion.

### 12.1 The Personas

| Persona         | Argues for                                          |
|-----------------|-----------------------------------------------------|
| The Optimist    | Maximal flexibility, fewer restrictions              |
| The Skeptic     | Safety, more constraints, edge cases                 |
| The Regulator   | Compliance, auditability, clear boundaries           |
| The Implementor | Simplicity, feasibility, performance                 |
| The Agent       | Clarity, predictability, unambiguous interpretation   |

### 12.2 The Referee

The debate requires a neutral **Referee** whose responsibilities are:

- Declaring when consensus is reached (all personas accept, no unresolved objections)
- Declaring exhaustion when the iteration limit is hit or debate becomes circular
- Documenting unresolved objections with structured classification
- Ensuring the Turing-completeness canary (Section 12.5) is applied at every round
- Preventing the debate from running indefinitely

The Referee does not advocate for any position. It manages process.

### 12.3 Debate Protocol

**Round 1 — Proposal:** Each persona independently submits candidate `facts.cue`, `entities.cue`, `rules.cue`, and `operations.cue` files.

**Round 2 — Critique:** Each persona reviews all other proposals, identifying:

- Missing facts or derived facts
- Underspecified entity states or transitions
- Ambiguous conditions
- Edge cases not covered
- Conflicts between rules
- Operations with unclear contracts
- Violations of the dependency graph
- Compositions that approach Turing completeness

**Round 3 — Synthesis:** Personas negotiate toward a unified proposal by either:

- Adopting one candidate as base with amendments
- Merging multiple candidates
- Identifying irreconcilable differences

**Round N — Iterate** until the Referee declares either:

- **Consensus:** All personas accept the resulting CUE files (the files type-check, all edge cases are covered, the dependency graph is respected, and no persona has unresolved objections)
- **Exhaustion:** The iteration limit is reached or debate has become circular, and at least one persona still has unresolved objections

### 12.4 The Exhaustion Signal

Exhaustion means: "We have CUE files that type-check but Persona X still says 'this doesn't handle case Y' and cannot be convinced otherwise."

The Referee classifies unresolved objections:

| Classification      | Meaning                                              | Action                        |
|---------------------|------------------------------------------------------|-------------------------------|
| `complexity`        | Domain is genuinely too complex for current model    | Simplify or defer             |
| `ambiguity`         | Requirements are unclear or contradictory            | Gather more information       |
| `scope`             | Edge case exists but is rare enough to exclude       | Document as out-of-scope      |
| `philosophical`     | Personas disagree on approach, not correctness       | Human decision required       |

The unresolved objections become documentation. They are not failures — they are a map of where the domain's complexity exceeds current understanding.

### 12.5 The Turing-Completeness Canary

At each round of debate, the Referee applies the following checks to the combined contract set:

- Can these primitives, composed together, express arbitrary computation?
- Do derived facts form cycles?
- Do rules reference other rules' outputs?
- Do flows contain inline logic that duplicates the rule engine?
- Has the standard library been extended with functions that enable unbounded recursion or iteration?

If any check fails, the offending construct must be split, restricted, or removed before debate continues.

---

## 13. Discovery

Agents discover Covenant systems via a well-known endpoint.

### 13.1 Well-Known URI

```
GET /.well-known/covenant
Authorization: Bearer <token-with-persona-claims>
Accept: text/x-cue
```

### 13.2 Response

```cue
{
  version:     "1.0"
  service:     "billing"
  description: "Handles invoicing, payments, and refunds"

  // Derived from authentication token
  persona: "billing_admin"

  // Static contract metadata (cacheable, changes only on deployment)
  contracts: {
    cue:            "/contracts/billing/"
    openapi:        "/generated/billing.openapi.json"
    stdlib_version: "0.1.0"
  }

  // Dynamic state (do not cache, fetch on demand)
  runtime: {
    active_flows: [
      "invoice-to-payment",
      "bulk-refund",
    ]
    snapshot_count: 3
  }
}
```

**Caching semantics:**

- `contracts` — cacheable. Changes only on deployment. Agents should re-fetch when notified of contract updates.
- `runtime` — not cacheable. Reflects current system state. Agents should fetch on demand.

### 13.3 Format Negotiation

The authoritative response is CUE. Agents that don't speak CUE can request JSON via `Accept: application/json`. The server converts on the fly. The CUE response is always canonical.

---

## 14. Implementation Layers (Non-Normative)

The following layer structure is one valid generation target from Covenant contracts. It is non-normative — other architectures may be generated from the same contracts.

```
┌────────────────────────────────────────────────┐
│              Generated Layer                    │
│        types, validators, transport             │
│       (generated from CUE + OpenAPI)            │
└──────────────┬───────────────────▲──────────────┘
               │                   │
            input            result | ErrorEnvelope
               │                   │
┌──────────────▼───────────────────┤──────────────┐
│              Handler Layer              ← ctx   │
│         orchestration, coordination             │
│       (receives flow snapshot context)          │
└──────────────┬───────────────────▲──────────────┘
               │                   │
            data in          domain result
               │                   │
┌──────────────▼───────────────────┤──────────────┐
│              Domain Layer                       │
│       (imports CUE types directly)              │
└──────────────┬───────────────────▲──────────────┘
               │                   │
           port call           response
               │                   │
┌──────────────▼───────────────────┤──────────────┐
│               Ports                             │
│        (abstract interfaces)                    │
└──────────────┬───────────────────▲──────────────┘
               │                   │
            invoke             return
               │                   │
┌──────────────▼───────────────────┤──────────────┐
│              Adapters                           │
│     (local mocks for dev,                       │
│      cloud adapters for prod)                   │
└─────────────────────────────────────────────────┘
```

Agents developing against contracts work with local adapters that mock all external dependencies. Cloud adapters are swapped in at deployment.

---

## 15. Generation Targets (Non-Normative)

From Covenant contracts, implementations can generate:

- **OpenAPI specifications** — for REST API consumers
- **Typed code** (TypeScript, Go, Java, etc.) — for type-safe implementation
- **Validation code** — for request/response validation
- **Documentation** — for human readers
- **Test suites** — for contract verification
- **State machine diagrams** — from entity definitions
- **DAG visualizations** — from the contract dependency graph
- **Persona capability matrices** — which personas can reach which operations

The contract repository is the source. Everything in this list is a disposable projection.

---

## 16. Development Workflow (Non-Normative)

1. Domain experts + AI personas debate contracts until consensus
2. Contracts are committed to the repository
3. Multiple AI agents generate complete applications from the same contracts
4. Stakeholders interact with generated applications, providing feedback
5. Feedback updates contracts (not code)
6. Regeneration produces the next iteration

The cost of seeing a working application approaches the cost of imagining it.

---

## 17. Versioning Discipline

Covenant follows Semantic Versioning (SemVer) while the specification is unstable (0.x).

### 17.1 Version Policy

- **0.x.y** — specification is unstable. Breaking changes may occur between minor versions.
- **1.0.0** — specification is stable. Breaking changes require a major version bump.
- Every version change includes a change log entry (see Appendix D).
- Breaking changes are declared explicitly with migration notes.

### 17.2 What Constitutes a Breaking Change

- Removing or renaming a normative section
- Changing the evaluation algorithm (Section 11)
- Changing the dependency graph direction (Section 3)
- Removing a verdict type
- Changing the authority model (Section 8.1)
- Altering the semantics of an existing standard library function

### 17.3 What Does Not Constitute a Breaking Change

- Adding new non-normative sections
- Adding new optional fields to existing structures
- Adding new functions to the standard library
- Clarifying ambiguous language without changing semantics
- Adding new appendix entries

### 17.4 Hardening Principles

During the 0.x phase, the following discipline applies:

- **Do not expand expressiveness.** The current primitive set should be tested against real domains before growing.
- **Do not add convenience features.** Convenience breeds implicit behavior.
- **Do not introduce implicit behavior.** Everything the system does must be traceable to a contract declaration.
- **Do not soften constraints for developer comfort.** The constraints are the product.
- **Expansion comes after hardening.** The 0.x phase is about tightening, not growing.

---

## 18. Glossary

| Term              | Definition                                                                 |
|-------------------|----------------------------------------------------------------------------|
| Covenant          | The mutual agreement between humans and AI, expressed through contracts    |
| Contract          | A machine-readable specification in CUE                                   |
| Fact              | A named, typed value the system knows about                               |
| Derived Fact      | A fact computed from other facts via the standard library                  |
| Entity            | A stateful object with a declared state machine                           |
| Rule              | A declarative condition that produces a verdict                           |
| Operation         | A named unit of work with declared input, output, errors, and constraints |
| Flow              | A sequence of operations that accomplish a goal                           |
| Persona           | An identity that can perform operations (single source of authorization)  |
| Snapshot          | A point-in-time view of rules for an active flow                          |
| Correlation ID    | A unique identifier linking all events within a flow instance             |
| Verdict           | The outcome of rule evaluation: deny, escalate, require, or flag          |
| Port              | An abstract interface to an external dependency                           |
| Adapter           | A concrete implementation of a port                                       |
| Standard Library  | The closed, versioned set of pure functions available for derived facts    |
| Exhaustion        | The state where AI personas cannot reach consensus, signaling complexity  |
| Referee           | The neutral persona that manages the debate protocol                      |
| Elevation         | Temporary, scoped, auditable assumption of a different persona            |

---

## 19. Summary

Covenant is a specification for building software where:

- Humans define what's possible through contracts
- AI agents execute faithfully within those bounds
- The contract dependency graph is a DAG that agents can fully traverse
- Entity lifecycles are declared as state machines and statically verifiable
- Operations are self-describing with complete input, output, and error contracts
- All conditional logic lives in the Rules layer, not scattered across flows
- Failure modes are part of the contract, including retry and fallback guidance
- Derived facts are explicit and constrained to a versioned standard library
- Persona boundaries are declared, auditable, and scoped
- Evaluation follows a strict, normative algorithm with side effects isolated to a single step
- Implementation is disposable, regenerated from contracts
- Domains that resist consensus are simplified or deferred, not forced

The name expresses the mutual obligation: Humans maintain accurate contracts. AI agents operate within them. Together, they build software neither could build alone.

---

## Appendix A: Open Questions (Unresolved by Design)

These are known tensions and gaps that are intentionally unresolved in v0.2. They are documented here so that ambiguity is visible, not hidden. Each entry names the concern, who raised it, and the current thinking.

### A.1 Snapshot Refresh Safety

**Raised by:** GPT, Grok

When a long-lived flow's snapshot expires and refreshes, the new rules may be incompatible with the flow's current state. What does "compatible" mean formally? Can a rule refresh change the set of valid next steps?

**Current position:** Snapshots can refresh, but the safety of mid-flow rule transitions is not formally specified. This is the hardest open problem in the spec.

**Needs:** A formal definition of snapshot compatibility. Possibly a diff algorithm that can determine whether new rules are safe to apply at a given flow step.

### A.2 Compound Verdict Presentation

**Raised by:** Grok

In compliance-heavy domains, 20+ rules may match a single operation. How are compound verdicts (5 flags + 2 requires + 1 escalate) presented to agents and humans?

**Current position:** All verdicts of the same type are returned. Multiple `require` verdicts must all be satisfied.

**Needs:** Clarity on maximum verdict counts, structured presentation formats for agents vs humans, and whether compound verdicts need their own envelope.

### A.3 Standard Library Growth Governance

**Raised by:** Grok, Gemini, GPT

Real domains will pressure the standard library to grow. Fuzzy matching, regex, geocoding, rate limiting, cryptographic verification — where is the boundary?

**Current position:** Additions go through the debate protocol. Functions must be pure, total, and terminating.

**Needs:** Formal criteria for what can and cannot be a stdlib function. Possibly tiered function categories with different trust levels. A Turing-completeness proof obligation for new additions.

### A.4 Cross-Domain Flow Orchestration

**Raised by:** Anticipated

What happens when a flow spans multiple domains (e.g., billing creates an invoice and shipping triggers fulfillment)?

**Current position:** Cross-domain facts must be lifted to Common Types. Cross-domain flows are not specified.

**Needs:** Decision on whether flows are strictly single-domain (with cross-domain coordination via events) or whether a meta-flow layer is warranted.

### A.5 Formal Snapshot Isolation Levels

**Raised by:** GPT

Snapshots are described narratively but not with formal isolation semantics. Possible levels:

- **Snapshot isolation** — flow sees rules as of start time (current default)
- **Refresh isolation** — flow may refresh rules at step boundaries
- **Strict isolation** — flow aborts if rules change during execution

**Current position:** Snapshot isolation with optional refresh on expiry. Formal definitions deferred.

### A.6 Verification and Testing

**Raised by:** Grok, GPT

How does one prove that generated implementation respects the contract? Regeneration safety is aspirational without verification.

**Current position:** Not specified.

**Needs:** Property-based testing from CUE constraints, symbolic execution of the evaluation algorithm, model checking of entity state machines, or contract conformance test suites. Likely a significant workstream.

### A.7 Precondition-Satisfiable Denies

**Raised by:** GPT

What if a deny is conditional on a fact that could be satisfied by a `require` rule? Currently rules are independent and non-mutating, so this situation cannot resolve through rule interplay.

**Current position:** This is safe but limits expressive power. Introducing rule interaction risks Turing creep.

**Needs:** Monitoring. If real domains consistently hit this pattern, a controlled mechanism may be needed. For now, the limitation is intentional.

### A.8 Observability and Traceability

**Raised by:** Grok

A `traceability_id` pattern for linking facts back to source documents/ports would strengthen auditability.

**Current position:** Not yet specified. Useful but unclear whether it belongs in core spec or as a non-normative extension.

### A.9 Cost and Risk Annotations

**Raised by:** Grok

Operations and rules could benefit from `cost_category` or `risk_tier` annotations for agent planning.

**Current position:** Potentially useful but domain-specific. Deferred until real usage patterns emerge.

---

## Appendix B: Future Directions (Non-Binding)

These are aspirations, not commitments. They are separated from the core spec so they cannot backdoor changes to normative sections.

### B.1 AI Reasoning Optimizations

Agents may benefit from pre-computed "capability surfaces" — materialized views of what a given persona can do, including transitive reachability through flows with persona crossings.

### B.2 Visual Graph Tooling

The contract DAG, entity state machines, and flow sequences are all naturally visual. Purpose-built tooling for rendering and navigating these structures would lower the barrier for non-technical stakeholders.

### B.3 Automated Rule Conflict Analysis

Static analysis that can detect potential rule conflicts, unreachable rules, and redundant conditions before deployment.

### B.4 Debate Protocol Formalization

The debate protocol (Section 12) could be formalized further: structured objection types, deterministic replay, quorum rules, and formal stopping criteria.

### B.5 Compliance-Grade Audit Generation

For regulated industries, generating audit-ready documentation from contracts — including rule justifications, persona boundary crossings, and elevation histories.

### B.6 Self-Hosting Governance

Expressing Covenant governance rules (who can modify common/, how stdlib grows, when domains split) as Covenant contracts themselves.

---

## Appendix C: Recommended Reviewers

After v0.2 is frozen, the most valuable external reviewers are not generalist LLMs or architecture enthusiasts. The spec needs pressure from:

- **Compliance engine builders** — they will stress the rule evaluation model and verdict semantics
- **DSL designers** — they will stress CUE's fitness and the standard library boundaries
- **Rule engine maintainers under regulation** — they will stress snapshot lifecycle and audit requirements
- **Teams that have migrated systems through breaking version shifts** — they will stress versioning discipline and migration paths

These reviewers will surface different failure modes than AI models can.

---

## Appendix D: Change Log

### v0.2.0 (2026-02-19)

**Breaking changes from v0.1.0:**

- Operations are now a first-class primitive in the dependency graph (new `operations.cue` file per domain)
- Entity state machines are now a first-class primitive (new `entities.cue` file per domain)
- Personas are the single source of authorization truth; the `restricted_to` field on operations has been replaced by `invocation_conditions`
- The `allow` verdict type has been removed; absence of blocking verdicts is permission
- Flow branching now uses `on_verdict` keys instead of inline `when` conditions
- The `@restricted` annotation is removed in favor of explicit `invocation_conditions`

**New normative sections:**

- Section 5: Entity State Machines
- Section 7: Operations (previously implicit, now declared)
- Section 8.1: Authority Model (single source of truth for authorization)
- Section 8.5: Elevation Protocol
- Section 9.4: Snapshot Lifecycle (TTL, expiration, correlation ID)
- Section 11: Evaluation Algorithm (normative step-by-step sequence)
- Section 17: Versioning Discipline

**New non-normative sections:**

- Section 3.2: Governance of Common Types
- Section 4.3: Derivation Standard Library with versioned function set
- Appendix A: Open Questions
- Appendix B: Future Directions
- Appendix C: Recommended Reviewers
- Appendix D: Change Log

**Enhancements:**

- Error envelopes now include `retry_after`, `fallback_operations`, and `human_escalation_fields`
- Discovery endpoint separates static contract metadata from dynamic runtime state with explicit caching semantics
- Debate protocol now includes a Referee persona and structured exhaustion classification
- Turing-completeness canary formalized as a standing check in the debate protocol
- Historical/aggregate facts documented with scalar presentation principle
- CUE portability note added (CUE is reference implementation, philosophy is portable)
- Hardening principles documented for the 0.x phase

### v0.1.0 (2026-02-19)

Initial specification.