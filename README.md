# Covenant

**Contract-First Systems for Human-AI Collaboration**

Covenant is a specification for building systems where humans and AI agents collaborate through machine-readable contracts. The contracts are the source of truth; implementation is disposable. Agents reason about what they can do by reading contracts, not code.

> The name expresses the mutual obligation: Humans maintain accurate contracts. AI agents operate within them. Together, they build software neither could build alone.

## Core Idea

In Covenant systems:

- **Contracts come first.** All system behavior is expressed in declarative contracts before any code is written. Implementation is derived from contracts, never the reverse.
- **Implementation is disposable.** Code can be fully regenerated at any time. Business logic lives in the contract layer, not in adapters or generated code.
- **AI agents are first-class citizens.** Contracts are structured for machine consumption first. An agent can determine what operations exist, what rules apply, what paths are available, and how to handle failure -- all without reading code.
- **The contract layer is decidable.** Contracts are non-Turing complete by design. No arbitrary computation, no implicit behavior, no surprises.

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

- **Common Types** -- shared type definitions across domains
- **Facts** -- named, typed values the system knows about, with declared sources
- **Entities** -- state machines with explicit transitions
- **Rules** -- declarative business policy that produces verdicts (deny, escalate, require, flag)
- **Operations** -- self-describing units of work with full input/output/error contracts
- **Flows** -- sequences of operations that accomplish a goal, scoped to a persona

No reverse dependencies. A Rule cannot define a Fact. An Operation cannot define a Rule. Each layer is independent.

## Authoritative Format

All contracts are written in [CUE](https://cuelang.org/), chosen for its unification of schema and data, built-in constraints, non-Turing completeness, and export to JSON/YAML/OpenAPI.

CUE is the reference implementation. The Covenant philosophy is portable to any language that satisfies the same constraints.

## Key Concepts

**Evaluation Algorithm** -- When an operation is invoked, the system follows a strict sequence: gather facts, derive computed facts, validate entity state, evaluate rules, apply verdicts, execute (side effects happen here and only here), transition state, advance flow. Steps before execution are side-effect-free.

**Personas** -- Identities that can perform operations. A persona's `can` list is the single source of authorization truth. Persona boundary crossings in flows are explicit and auditable.

**Snapshots** -- When an agent begins a flow, it receives a point-in-time snapshot of rules. The agent operates in a consistent logical universe mid-flow. Rules can evolve without breaking in-progress work.

**Design by Debate** -- Before a domain is implemented, AI personas (Optimist, Skeptic, Regulator, Implementor, Agent) debate its contracts until consensus or exhaustion. Unresolved objections become documentation, not hidden assumptions.

## Status

**v0.2.0** -- The specification is unstable. Breaking changes may occur between minor versions. The 0.x phase is about tightening, not growing.

See the full specification in [COVENANT.md](COVENANT.md).

## Contributors

| Name | Role |
|------|------|
| Brandon Bush | Author |
| Claude (Anthropic) | Contributor |
| DeepSeek | Contributor |
| GPT (OpenAI) | Contributor |
| Grok (xAI) | Contributor |
| Gemini (Google) | Contributor |

## License

[The Unlicense](LICENSE) -- public domain. Do whatever you want with it.
