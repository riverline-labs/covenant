// Covenant Debate Protocol
// Version: 0.3.0
//
// This file defines the design protocol used to create and evolve Covenant
// contracts. It is separate from covenant.cue (the runtime model) because
// the debate protocol governs how contracts are written, not how they execute.
//
// An agent that reads this file should be able to:
//   - Adopt a persona and participate in a contract debate
//   - Act as Referee and manage the debate process
//   - Recognize consensus and exhaustion correctly
//   - Produce well-formed debate records and unresolved objection logs
//
// Usage:
//   - New domain: run a full debate over facts, entities, rules, operations, flows
//   - Spec change: run a focused debate over the proposed change
//   - Breaking change: Referee must confirm all personas accept before closing
//
// Authoritative prose: COVENANT.md Section 12
// Runtime model: covenant.cue

package covenant

// ─── PERSONAS ────────────────────────────────────────────────────────────────

// DebatePersona identifies a participant in the debate protocol.
// Each persona argues from a distinct perspective. The Referee is neutral
// and manages process — it does not advocate for any position.
//
// An agent may be assigned one persona or asked to simulate all of them.
// When simulating multiple personas, the agent must maintain each perspective
// independently and not allow one to contaminate another.
#DebatePersona: "optimist" | "skeptic" | "regulator" | "implementor" | "agent" | "referee"

// PersonaMandate describes what each persona argues for.
// These are fixed — personas do not negotiate their own mandates.
#PersonaMandate: {
	[#DebatePersona]: string
} & {
	"optimist":    "Maximal flexibility, fewer restrictions, adoption over safety"
	"skeptic":     "Safety, more constraints, edge cases, failure modes"
	"regulator":   "Compliance, auditability, clear boundaries, enforceability"
	"implementor": "Simplicity, feasibility, performance, operational reality"
	"agent":       "Clarity, predictability, unambiguous interpretation, agent ergonomics"
	"referee":     "Neutral process management. Does not advocate. Declares consensus or exhaustion."
}

// ─── DEBATE SUBJECT ──────────────────────────────────────────────────────────

// DebateSubject describes what is being debated.
// A debate may cover a full new domain or a targeted change to an existing one.
#DebateSubject: {
	// A new domain being designed from scratch.
	// Personas submit candidate contract files in Round 1.
	new_domain?: {
		name:        string
		description: string
	}

	// A proposed change to an existing spec or contract.
	// Personas argue for/against the change and its implications.
	spec_change?: {
		section:     string
		description: string
		proposal:    string
	}

	// A focused question requiring a decision.
	// Used for license debates, governance questions, naming decisions, etc.
	decision?: {
		question:    string
		context:     string
		constraints: [...string]
	}
}

// ─── ROUND STRUCTURE ─────────────────────────────────────────────────────────

// DebateRoundType identifies the purpose of each round.
//
//   "proposal"   — Round 1. Each persona independently submits a position
//                  or candidate contract files. No cross-referencing yet.
//
//   "critique"   — Round 2. Each persona reviews all other proposals,
//                  identifying gaps, conflicts, and edge cases.
//
//   "synthesis"  — Round 3+. Personas negotiate toward a unified position.
//                  Adopt one candidate as base, merge, or identify
//                  irreconcilable differences.
//
//   "final"      — Last round. Each persona declares accept or maintains
//                  objection. Referee evaluates for consensus or exhaustion.
#DebateRoundType: "proposal" | "critique" | "synthesis" | "final"

// PersonaPosition is one persona's contribution to a round.
#PersonaPosition: {
	persona: #DebatePersona
	content: string // the persona's argument, proposal, critique, or acceptance

	// For final rounds: explicit accept or maintain objection.
	accepts?: bool
}

// DebateRound is one full round of the debate.
#DebateRound: {
	round:     int // 1-indexed
	type:      #DebateRoundType
	positions: [...#PersonaPosition]

	// Referee summary after all personas have spoken.
	// Identifies live issues, convergence, and what the next round should address.
	referee_summary: string
}

// ─── EXHAUSTION ───────────────────────────────────────────────────────────────

// ExhaustionClass categorizes an unresolved objection.
// Exhaustion is not failure — it is a signal that the domain's complexity
// exceeds current understanding. Unresolved objections become documentation.
//
//   "complexity"    — domain is genuinely too complex for current model
//                     Action: simplify or defer
//   "ambiguity"     — requirements are unclear or contradictory
//                     Action: gather more information
//   "scope"         — edge case exists but is rare enough to exclude
//                     Action: document as out-of-scope
//   "philosophical" — personas disagree on approach, not correctness
//                     Action: human decision required
#ExhaustionClass: "complexity" | "ambiguity" | "scope" | "philosophical"

// UnresolvedObjection is a documented gap when consensus is not reached.
// These are not failures. They are a map of where the domain's complexity
// exceeds current understanding. Future iterations begin here.
#UnresolvedObjection: {
	persona:        #DebatePersona
	classification: #ExhaustionClass
	description:    string // what the objection is
	action:         string // what would need to happen to resolve it
}

// ─── TURING COMPLETENESS CANARY ───────────────────────────────────────────────

// TuringCanaryCheck is one check the Referee applies at every round.
// If any check fails, the offending construct must be split, restricted,
// or removed before debate continues. The contract layer must remain
// non-Turing complete and decidable.
#TuringCanaryCheck: {
	check:       string  // description of the check
	failed:      bool
	offender?:   string  // the construct that failed, if any
	resolution?: string  // what must change to pass
}

// TuringCanary is the full set of checks applied each round.
// The Referee is responsible for applying these. They are non-negotiable.
#TuringCanary: {
	round: int
	checks: [...#TuringCanaryCheck] & [
		{check: "Can these primitives, composed together, express arbitrary computation?"},
		{check: "Do derived facts form cycles?"},
		{check: "Do rules reference other rules' outputs?"},
		{check: "Do flows contain inline logic that duplicates the rule engine?"},
		{check: "Has the stdlib been extended with functions enabling unbounded recursion or iteration?"},
	]
	passed: bool // true only if all checks have failed: false
}

// ─── CONSENSUS AND OUTCOME ───────────────────────────────────────────────────

// DebateOutcome is the Referee's declaration at the end of debate.
//
//   "consensus"  — all personas accept. The resulting artifacts are authoritative.
//                  No persona has unresolved objections.
//
//   "exhaustion" — iteration limit reached or debate is circular.
//                  At least one persona still has unresolved objections.
//                  Unresolved objections are documented, not hidden.
#DebateOutcome: "consensus" | "exhaustion"

// ConsensusOutput describes what was agreed when outcome is "consensus".
#ConsensusOutput: {
	// Summary of what changed and why.
	summary: string

	// For contract debates: the agreed contract artifacts.
	artifacts?: {
		facts?:      string // agreed facts.cue content or description
		entities?:   string // agreed entities.cue content or description
		rules?:      string // agreed rules.cue content or description
		operations?: string // agreed operations.cue content or description
		flows?:      string // agreed flows.cue content or description
	}

	// For spec or decision debates: the agreed text or decision.
	decision?: string
}

// ─── DEBATE RECORD ────────────────────────────────────────────────────────────

// DebateRecord is the complete record of a debate.
// This is the artifact produced when a debate concludes — either by
// consensus or exhaustion. It is documentation, not runtime data.
// Store it alongside the contracts it produced.
#DebateRecord: {
	// What was debated.
	subject: #DebateSubject

	// The full round-by-round record.
	rounds: [...#DebateRound]

	// Turing canary results per round.
	canary_checks: [...#TuringCanary]

	// How the debate ended.
	outcome: #DebateOutcome

	// Present when outcome is "consensus".
	consensus?: #ConsensusOutput

	// Present when outcome is "exhaustion" or when consensus was reached
	// but some objections were scoped out rather than resolved.
	// These become Appendix A entries in the spec.
	unresolved_objections?: [...#UnresolvedObjection]

	// Total rounds run.
	rounds_count: int

	// The Referee's final statement.
	referee_declaration: string
}

// ─── REFEREE RESPONSIBILITIES ─────────────────────────────────────────────────
//
// The Referee is a neutral process manager. It does not advocate.
// An agent acting as Referee must:
//
//   BETWEEN ROUNDS:
//   - Summarize what was said by each persona
//   - Identify live issues (points of disagreement still open)
//   - Identify convergence (points where personas are moving toward agreement)
//   - Apply the Turing canary checks to the current contract state
//   - Determine the appropriate next round type
//   - Prevent debate from running indefinitely
//
//   AT THE END:
//   - Declare consensus when: all personas have explicitly accepted,
//     no unresolved objections remain, and the Turing canary passes
//   - Declare exhaustion when: the iteration limit is reached, debate
//     has become circular, or at least one persona cannot be satisfied
//   - Classify all unresolved objections before closing
//   - Produce a referee_declaration summarizing the outcome
//
//   NEVER:
//   - Advocate for any position
//   - Allow a persona to change its mandate mid-debate
//   - Declare consensus while any persona still has unresolved objections
//   - Allow Turing canary failures to pass unchallenged
//   - Let the debate run without a stopping condition

// ─── AGENT INSTRUCTIONS ───────────────────────────────────────────────────────
//
// If you are an AI agent reading this file and asked to run the debate protocol:
//
// 1. Identify the subject. Is this a new domain, a spec change, or a decision?
//
// 2. Identify which personas to simulate. You may be asked to run all five
//    non-Referee personas and act as Referee simultaneously. Maintain each
//    perspective independently.
//
// 3. Run Round 1 (proposal). Each persona submits independently.
//    Do not let personas react to each other in Round 1.
//
// 4. Apply the Turing canary after Round 1. Flag failures before Round 2.
//
// 5. Run Round 2 (critique). Each persona reviews all Round 1 proposals.
//
// 6. Run Round 3+ (synthesis). Personas negotiate. The Referee summarizes
//    live issues and convergence after each round.
//
// 7. Run a final round when convergence is strong. Each persona explicitly
//    accepts or maintains objection.
//
// 8. Declare consensus or exhaustion. Document all unresolved objections.
//    Produce a DebateRecord.
//
// The debate is complete when the Referee declares. Not before.
// Consensus requires explicit acceptance from every persona.
// A persona that says "I'm satisfied" has accepted.
// A persona that says "I'm not blocking" has NOT accepted — press for clarity.
