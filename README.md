# Covenant

**Contract-First Systems for Human-AI Collaboration**

Covenant is a specification for building systems where humans and AI agents collaborate through machine-readable contracts. The contracts are the source of truth; implementation is disposable. Agents reason about what they can do by reading contracts, not code.

> The name expresses the mutual obligation: Humans maintain accurate contracts. AI agents operate within them. Together, they build software neither could build alone.

## Core Idea

In Covenant systems:

- **Contracts come first.** All system behavior is expressed in declarative contracts before any code is written. Implementation is derived from contracts, never the reverse.
- **Implementation is disposable.** The normative runtime is a generic executor that interprets contracts directly — there is no generated code capable of drifting from the contract source. Regeneration-safety is structural, not disciplinary.
- **AI agents are first-class citizens.** Contracts are structured for machine consumption first. An agent can determine what operations exist, what rules apply, what paths are available, and how to handle failure — all without reading code.
- **The contract layer is decidable.** Contracts are non-Turing complete by design. No arbitrary computation, no implicit behavior, no surprises.

## Architecture

Three layers. Each evolves independently, owned by different teams, on different schedules.

```
┌──────────────────────────────────────────────────────────────┐
│                      CONTRACT LAYER                          │
│  (CUE files, Git, CDN — static, authoritative)               │
│                                                              │
│  • facts.cue      — what the system knows                    │
│  • entities.cue   — valid state machines                     │
│  • rules.cue      — business policy                          │
│  • operations.cue — what you can do                          │
│  • flows.cue      — how to accomplish goals                  │
└────────────────────────────┬─────────────────────────────────┘
                             │ loaded by
                             ▼
┌──────────────────────────────────────────────────────────────┐
│                     EXECUTION FLEET                          │
│  (Stateless orchestrator — the conductor)                    │
│                                                              │
│  • Loads contracts from CDN                                  │
│  • Authenticates requests                                    │
│  • Gathers facts         (calls adapters)                    │
│  • Evaluates rules                                           │
│  • Resolves verdicts                                         │
│  • Executes operations   (calls adapters)                    │
│  • Returns responses                                         │
│  • Writes audit logs                                         │
│                                                              │
│       [Instance 1]   [Instance 2]   [Instance 3]  ...        │
└────────────────────────────┬─────────────────────────────────┘
              ┌──────────────┼──────────────┐
              │              │              │
              ▼              ▼              ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│    ADAPTERS     │ │    ADAPTERS     │ │    ADAPTERS     │
│  (The workers)  │ │  (The workers)  │ │  (The workers)  │
├─────────────────┤ ├─────────────────┤ ├─────────────────┤
│ customerRepo    │ │ pdfExtractor    │ │ paymentProc     │
│ • Postgres      │ │ • Textract      │ │ • Stripe        │
│ • Go service    │ │ • Bedrock       │ │ • Go library    │
│ • Scales with   │ │ • Python on ECS │ │ • In-process    │
│   query load    │ │ • Scales with   │ │ • Scales with   │
│                 │ │   file size     │ │   transaction   │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

| Concern | Lives In | Can Change |
|---|---|---|
| What the business does | Contracts | Hourly, by anyone |
| How execution happens | Executor | Rarely, by platform team |
| How data is stored | Adapters | Per adapter, by owner |
| How AI is used | Adapters | Per model, by ML team |
| How users interact | Clients | Per channel, by frontend team |

## The Contract Stack

Contracts are organized in a strict dependency DAG. Each layer can be understood, validated, and generated independently.

```
Common Types    (money.cue, customer.cue, error.cue)
     |
Domain Facts    (billing/facts.cue)
     |
Entity States   (billing/entities.cue)
     |
Domain Rules    (billing/rules.cue)
     |
Operations      (billing/operations.cue)
     |
Flows           (billing/flows.cue)
```

- **Common Types** — shared type definitions across domains
- **Facts** — named, typed values the system knows about, with declared sources
- **Entities** — state machines with explicit transitions
- **Rules** — declarative business policy that produces verdicts (deny, escalate, require, flag)
- **Operations** — self-describing units of work with full input/output/error contracts
- **Flows** — sequences of operations that accomplish a goal, scoped to a persona

No reverse dependencies. A Rule cannot define a Fact. An Operation cannot define a Rule. Each layer is independent.

## Authoritative Format

All contracts are written in [CUE](https://cuelang.org/), chosen for its unification of schema and data, built-in constraints, non-Turing completeness, and export to JSON/YAML/OpenAPI.

The machine-readable specification lives in [`covenant.cue`](covenant.cue) — it defines the schema for all Covenant contract files and annotates each field with enough semantic context for an AI agent to understand not just the shape of a valid contract but the intent behind it. An agent that reads `covenant.cue` can validate contracts, understand the evaluation algorithm, and reason about the system without reading the prose spec.

CUE is the reference implementation. The Covenant philosophy is portable to any language that satisfies the same constraints.

## Key Concepts

**Generic Executor** — The normative runtime model. A single `POST /execute` endpoint interprets contracts directly at runtime. There are no operation-specific handlers and no code generation pipeline. Business logic lives exclusively in contracts; the only irreducible implementation artifacts are port adapters.

**Evaluation Algorithm** — When an operation is invoked, the executor follows a strict sequence: gather facts, derive computed facts, validate entity state, evaluate rules, apply verdicts, execute (side effects happen here and only here), transition state, advance flow. Steps before execution are side-effect-free.

**Personas** — Identities that can perform operations. A persona's `can` list is the single source of authorization truth. Persona boundary crossings in flows are explicit and auditable.

**Snapshots** — When an agent begins a flow, it receives a point-in-time snapshot of rules. The agent operates in a consistent logical universe mid-flow. Rules can evolve without breaking in-progress work.

**Design by Debate** — Before a domain is implemented, AI personas (Optimist, Skeptic, Regulator, Implementor, Agent) debate its contracts until consensus or exhaustion. Unresolved objections become documentation, not hidden assumptions. The debate protocol is defined in [`debate.cue`](debate.cue) — an agent can read it and run a debate without consulting the prose spec.

## Examples

- [**examples/go**](examples/go) — A complete Go implementation: contract server, generic executor, and CLI client, using the CUE Go SDK to interpret contracts at runtime.

## Status

**v0.3.0** — The specification is unstable. Breaking changes may occur between minor versions. The 0.x phase is about tightening, not growing.

| File | Purpose |
|---|---|
| [COVENANT.md](COVENANT.md) | Full prose specification |
| [covenant.cue](covenant.cue) | Machine-readable spec — contract schema, evaluation algorithm, execution model |
| [debate.cue](debate.cue) | Machine-readable debate protocol — personas, rounds, consensus, exhaustion |

The CUE files are designed so an AI agent can understand and work with Covenant without consuming the full prose spec.

## Contributors

| Name | Role |
|---|---|
| Brandon W. Bush | Originator |
| Claude (Anthropic) | Contributor |
| DeepSeek | Contributor |
| GPT (OpenAI) | Contributor |
| Grok (xAI) | Contributor |
| Gemini (Google) | Contributor |

## License

[Creative Commons Attribution 4.0 International (CC BY 4.0)](LICENSE) — free to share and adapt with attribution.
