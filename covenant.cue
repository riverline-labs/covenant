// Covenant — Contract-First Systems for Human-AI Collaboration
// Version: 0.3.0
//
// This file is the machine-readable form of the Covenant specification.
// It defines the schema for all Covenant contract files and annotates
// each field with enough semantic context for an AI agent to understand
// not just the shape of a valid contract but the intent behind it.
//
// An agent that reads this file should be able to:
//   - Validate that a set of contract files is well-formed
//   - Understand what each field means and how it affects execution
//   - Reason about the evaluation algorithm without reading COVENANT.md
//
// Authoritative prose specification: COVENANT.md
// Reference implementation: examples/go
//
// License: CC BY 4.0

package covenant

// ─── STDLIB ──────────────────────────────────────────────────────────────────

// StdlibVersion is the version of the derivation standard library in use.
// Contracts must declare which version they depend on. Functions may be
// deprecated but never removed while any active contract references them.
#StdlibVersion: string

// StdlibFunction defines a pure, total, terminating function available
// for use in derived fact derivations. Functions must not perform I/O,
// access ports, or reference external state.
#StdlibFunction: {
	args:        1 | 2
	returns:     "bool" | "number" | "string"
	description?: string
}

// The closed set of derivation functions. This set can only grow through
// the debate protocol (Section 12). Turing creep is prevented by requiring
// every function to be pure, total, and terminating.
#Stdlib: {
	stdlib_version: #StdlibVersion

	functions: {
		// Comparison
		"equals":           #StdlibFunction & {args: 2, returns: "bool"}
		"not_equals":       #StdlibFunction & {args: 2, returns: "bool"}
		"greater_than":     #StdlibFunction & {args: 2, returns: "bool"}
		"greater_or_equal": #StdlibFunction & {args: 2, returns: "bool"}
		"less_than":        #StdlibFunction & {args: 2, returns: "bool"}
		"less_or_equal":    #StdlibFunction & {args: 2, returns: "bool"}

		// Arithmetic
		"add":      #StdlibFunction & {args: 2, returns: "number"}
		"subtract": #StdlibFunction & {args: 2, returns: "number"}
		"multiply": #StdlibFunction & {args: 2, returns: "number"}
		"divide":   #StdlibFunction & {args: 2, returns: "number"}
		"modulo":   #StdlibFunction & {args: 2, returns: "number"}

		// Date/Time
		"date_before":    #StdlibFunction & {args: 2, returns: "bool"}
		"date_after":     #StdlibFunction & {args: 2, returns: "bool"}
		"date_diff_days": #StdlibFunction & {args: 2, returns: "number"}

		// String
		"contains":    #StdlibFunction & {args: 2, returns: "bool"}
		"starts_with": #StdlibFunction & {args: 2, returns: "bool"}
		"ends_with":   #StdlibFunction & {args: 2, returns: "bool"}
		"to_lower":    #StdlibFunction & {args: 1, returns: "string"}
		"to_upper":    #StdlibFunction & {args: 1, returns: "string"}

		// Collection
		"in":       #StdlibFunction & {args: 2, returns: "bool", description: "Value is member of list"}
		"length":   #StdlibFunction & {args: 1, returns: "number"}
		"is_empty": #StdlibFunction & {args: 1, returns: "bool"}

		// Logical
		"and": #StdlibFunction & {args: 2, returns: "bool"}
		"or":  #StdlibFunction & {args: 2, returns: "bool"}
		"not": #StdlibFunction & {args: 1, returns: "bool"}
	}
}

// ─── FACTS ───────────────────────────────────────────────────────────────────

// FactSource declares where a base fact comes from.
//   "input"     — provided in the operation's request body
//   "ctx"       — provided by the execution context (auth, tenant, etc.)
//   "port:NAME" — fetched from a named port adapter before evaluation begins
//
// input and ctx facts that are absent are schema validation failures,
// caught before fact-gathering begins. on_missing does not apply to them.
// on_missing applies exclusively to port facts.
#FactSource: "input" | "ctx" | =~"^port:.+"

// OnMissing governs executor behavior when a port fact cannot be gathered.
// Defaults to "system_error" if not declared.
//
//   "system_error" — terminate invocation immediately with FACT_UNAVAILABLE.
//                    No verdict is produced. Use for facts that are required
//                    for any meaningful evaluation to proceed.
//   "deny"         — produce a denied outcome with FACT_UNAVAILABLE.
//                    FACT_UNAVAILABLE must be distinguishable from a
//                    business-rule denial in both response and audit record.
//   "skip"         — treat the fact as absent. Rule predicates referencing
//                    this fact evaluate to false. Use for optional enrichment
//                    facts whose absence is a valid operational state.
#OnMissing: "system_error" | "deny" | "skip"

// FactDef declares a named, typed value the system knows about.
// Facts are immutable for the duration of a single evaluation.
// The fact set is closed — no facts can be added during evaluation.
// Every base fact must declare its source.
#FactDef: {
	source:       #FactSource
	required?:    bool | *true
	on_missing?:  #OnMissing | *"system_error"
	description?: string
}

// DerivationArg is one argument to a derivation function.
// Exactly one of fact or value must be set.
//   fact  — reference to another fact in the fact set (base or derived)
//   value — a literal value
#DerivationArg: {
	fact?:  string
	value?: _
}

// Derivation defines how a derived fact is computed.
// fn must be a function name from the stdlib.
// args are evaluated left to right; fact references are resolved
// from the current fact set at evaluation time.
// Derived facts must form a DAG — no cycles are permitted.
#Derivation: {
	fn:   string
	args: [...#DerivationArg]
}

// DerivedFactDef declares a fact that is computed from other facts
// via the standard library. Derived facts are evaluated in topological
// order before rules execute, then treated as ordinary facts.
// Derived facts are deterministic: same inputs always produce same output.
#DerivedFactDef: {
	derivation:   #Derivation
	description?: string
}

// ─── ENTITIES ─────────────────────────────────────────────────────────────────

// EntityTransition declares one valid state change for an entity.
// Every valid transition must name its source state, target state,
// and the operation that triggers it.
// Use "*" as from to mean "any non-terminal state" (requires a guard).
#EntityTransition: {
	from: string | "*"
	to:   string
	via:  string // operation name
	guard?: {
		not_in: [...string]
	}
}

// EntityDef declares a stateful object with an explicit state machine.
// Undeclared states are invalid. Every valid transition is enumerated.
// Tooling must verify that all states are reachable and that terminal
// states are reachable from all non-terminal states.
#EntityDef: {
	// All valid states. Undeclared states are invalid.
	states: [...string]

	// The state an entity begins in.
	initial: string

	// States from which no further transitions are possible.
	terminal: [...string]

	// All valid transitions. A transition not listed here is invalid.
	transitions: [...#EntityTransition]
}

// ─── RULES ───────────────────────────────────────────────────────────────────

// Condition is a declarative predicate evaluated against the fact set.
// Conditions use only all/any/not combinators and comparison operators.
// No computed facts. No rule-to-rule dependencies. No mutation.
// Evaluation order does not matter — rules are independent.
#Condition: {
	// Leaf condition: evaluate a single fact
	fact?:         string
	equals?:       _
	not_equals?:   _
	greater_than?:  number
	less_than?:    number
	in?:           [..._]

	// Composite conditions
	all?: [...#Condition]
	any?: [...#Condition]
	not?: #Condition
}

// ErrorEnvelope is the structured error returned to callers.
// Every known failure mode is declared in advance so agents can
// plan for retry, fallback, and escalation without trial and error.
#ErrorEnvelope: {
	code:       string
	message?:   string
	httpStatus: int
	category:   "validation" | "business_rule_violation" | "authorization" | "system" | "external_dependency"
	retryable:  bool

	// Present when retryable is true. ISO 8601 duration e.g. "PT5M"
	retry_after?: string

	// Recovery guidance for agents
	suggestion?:              string
	fallback_operations?:     [...string]
	human_escalation_fields?: [...string]

	details?: {...}
}

// VerdictType expresses the outcome of rule evaluation.
// Priority when multiple rules match: deny > escalate > require > flag
// There is no explicit "allow" — absence of blocking verdicts is permission.
// Rules are independent: a require cannot satisfy a deny.
#VerdictType: "deny" | "escalate" | "require" | "flag"

// DenyVerdict blocks the operation entirely. No side effects occur.
// The error envelope is returned to the caller.
#DenyVerdict: {
	code:   string
	reason: string
	error:  #ErrorEnvelope
}

// EscalateVerdict queues the operation for human review. No side effects occur.
#EscalateVerdict: {
	queue:  string
	reason: string
}

// RequireVerdict returns additional conditions to the agent.
// The agent must satisfy these before the operation can proceed.
// No side effects occur.
#RequireVerdict: {
	conditions: [...string]
	reason:     string
}

// FlagVerdict attaches a warning but does not block execution.
// The operation proceeds to the execute step.
#FlagVerdict: {
	code:   string
	reason: string
}

// VerdictDef is the outcome declared by a rule when its condition matches.
// Exactly one verdict type should be set.
#VerdictDef: {
	deny?:     #DenyVerdict
	escalate?: #EscalateVerdict
	require?:  #RequireVerdict
	flag?:     #FlagVerdict
}

// RuleDef encodes a business policy. Rules are declarative, non-Turing
// complete, and side-effect-free. Rule evaluation never triggers side
// effects. Deny and escalate verdicts never cause partial execution.
//
// applies_to links rules to operations via the operation's constrained_by
// field. Only rules listed in an operation's constrained_by are evaluated
// for that operation.
#RuleDef: {
	id:          string
	applies_to:  [...string] // operation names
	when:        #Condition
	verdict:     #VerdictDef
	description?: string
}

// ─── OPERATIONS ──────────────────────────────────────────────────────────────

// EntityTransitionRef declares which entity state transition an operation triggers.
#EntityTransitionRef: {
	entity: string
	from?:  string
	to:     string
}

// InvocationCondition declares additional steps required when a specific
// persona invokes this operation. This is not authorization — the persona
// already has the operation in its can list. It constrains how the
// operation is invoked by that persona.
#InvocationCondition: {
	requires:       "approval" | "mfa" | string
	approval_from?: string // persona name
	reason:         string
}

// OperationDef is the verb of the system. Each operation is self-describing:
// an agent can read one operation and know its full interface without
// consulting other files.
//
// Side effects occur via ports in the execute step, and only if all rules
// permit execution. Steps 1-5 of the evaluation algorithm are side-effect-free.
#OperationDef: {
	// Input schema. CUE types constrain what goes in.
	input: {...}

	// Output schema. CUE types constrain what comes out.
	output: {...}

	// Every known failure mode declared in advance.
	// Agents plan retry, fallback, and escalation from this list.
	errors: [...#ErrorEnvelope]

	// Rule IDs that constrain this operation.
	// Only rules listed here are evaluated when this operation is invoked.
	// Cross-referenced with applies_to in rules.cue.
	constrained_by: [...string]

	// Entity state transitions this operation triggers.
	// Declared transitions are validated against entities.cue by tooling.
	transitions: [...#EntityTransitionRef]

	// Per-persona invocation constraints. Not authorization — that lives
	// in personas.cue. This constrains how a persona invokes the operation.
	invocation_conditions?: {
		[persona=string]: #InvocationCondition
	}

	min_executor_version?: string
}

// ─── PERSONAS ────────────────────────────────────────────────────────────────

// PersonaDef is an identity that can perform operations.
// Personas are the single source of authorization truth.
// A persona's can list is the authoritative declaration of what operations
// that persona may invoke. Operations do not independently declare who
// can use them.
//
// Tooling must verify that every operation referenced in any flow is
// present in the can list of the flow's persona. An operation that no
// persona can invoke is dead code.
#PersonaDef: {
	description:   string
	can:           [...string] // operation names
	requires_mfa?: bool
}

// ─── FLOWS ───────────────────────────────────────────────────────────────────

// FlowStepTransition declares the entity state change a flow step produces.
#FlowStepTransition: {
	entity: string
	state:  string
}

// FlowStep is one operation invocation within a flow.
// requires and produces are validated against entities.cue by tooling —
// a flow that attempts an invalid transition is a contract error.
#FlowStep: {
	operation: string

	// Entity state required before this step can execute.
	requires?: #FlowStepTransition

	// Entity state produced after this step executes.
	produces?: #FlowStepTransition

	// Persona to use for this step if different from the flow's persona.
	// Every persona boundary crossing is explicit and auditable.
	as?:          string // persona name
	reason?:      string
	requires_mfa?: bool

	// Verdict-driven branching. Branch keys are verdict outcomes.
	// All conditional logic lives in the Rules layer — flows describe
	// sequence, not logic. Every branch must produce a valid transition.
	on_verdict?: {
		[verdict=string]: {
			operation: string
			produces?: #FlowStepTransition
		}
	}
}

// OnExpiry declares what happens when a flow snapshot's TTL expires.
//   "refresh"      — create a new snapshot from current rules, resume from current step
//   "abort"        — terminate the flow
//   "human_review" — escalate to human before deciding
#OnExpiry: "refresh" | "abort" | "human_review"

// FlowDef is a sequence of operations that accomplishes a goal.
// Flows are the unit of agent work. They have a primary persona and
// may cross persona boundaries explicitly at named steps.
//
// When an agent begins a flow, it receives a snapshot of the rules
// as they existed at that moment. The agent operates in a consistent
// logical universe mid-flow. Rules can evolve without breaking
// in-progress work.
#FlowDef: {
	id:      string
	persona: string // primary persona name
	goal:    string // human-readable description of what this flow accomplishes

	steps: [...#FlowStep]

	// Maximum time-to-live for the flow snapshot.
	// ISO 8601 duration e.g. "P7D" (7 days), "P90D" (90 days)
	snapshot_ttl?: string

	// What to do when the snapshot TTL expires mid-flow.
	on_expiry?: #OnExpiry | *"abort"
}

// ─── DOMAIN CONTRACT ─────────────────────────────────────────────────────────

// DomainContract is the complete contract set for one domain.
// In practice these are split across multiple .cue files per the
// repository structure in Section 3.1, but this definition describes
// the logical whole that the executor loads and evaluates against.
//
// Dependency direction is strict and acyclic:
//   flows → operations → rules → entities → facts → common types
// No reverse dependencies are permitted.
#DomainContract: {
	package: string

	// Minimum executor version required to evaluate this contract.
	// Executors that cannot satisfy this must reject all requests
	// with EXECUTOR_VERSION_INCOMPATIBLE.
	min_executor_version?: string

	// Version of the stdlib this contract depends on.
	stdlib_version: #StdlibVersion

	facts:         {[name=string]: #FactDef}
	derived_facts: {[name=string]: #DerivedFactDef}
	entities:      {[name=string]: #EntityDef}
	rules:         [...#RuleDef]
	operations:    {[name=string]: #OperationDef}
	flows:         [...#FlowDef]
	personas:      {[name=string]: #PersonaDef}
}

// ─── EVALUATION ALGORITHM ─────────────────────────────────────────────────────
//
// The executor follows this sequence for every operation invocation.
// This is normative — no operation-specific logic exists outside this algorithm.
// Steps 1-5 are side-effect-free. Side effects occur only in step 6.
//
// 1. GATHER base facts
//    - input facts: validate against operation input schema
//    - ctx facts: collect from execution context
//    - port facts: fetch from named port adapters; apply on_missing policy on failure
//
// 2. DERIVE computed facts
//    - topologically sort derived_facts by dependency
//    - evaluate each using stdlib functions
//    - add results to the fact set
//
// 3. VALIDATE entity state
//    - check operation's required entity state matches current state
//    - check target transition is valid per entities definition
//    - if invalid: return error, do not proceed
//
// 4. EVALUATE rules
//    - select rules where id is in operation's constrained_by list
//    - evaluate each rule's when condition against the fact set
//    - collect all matching verdicts
//    - resolve conflicts: deny > escalate > require > flag
//
// 5. APPLY verdict
//    - deny:     return error envelope, no side effects
//    - escalate: queue for human review, no side effects
//    - require:  return required conditions to agent, no side effects
//    - flag:     attach warnings, proceed to step 6
//    - none:     proceed to step 6
//
// 6. EXECUTE operation
//    - invoke operation logic via port adapters
//    - side effects occur here and only here
//    - return output
//
// 7. TRANSITION entity state
//    - update entity state per declared transition
//    - record state change
//
// 8. ADVANCE flow
//    - update flow instance current step
//    - if on_verdict branching: select branch
//    - if final step: complete flow, garbage-collect snapshot

// ─── DISCOVERY ───────────────────────────────────────────────────────────────

// DiscoveryResponse is the response from GET /.well-known/covenant.
// The executor serves this endpoint. Agents use it to discover the system,
// resolve the active persona, and detect contract changes via contract_etag.
//
// contract_etag MUST change whenever any contract in the domain changes.
// Agents that cache contracts MUST re-fetch when contract_etag changes.
// Agents MUST NOT operate on contracts they know to be stale.
// Polling this endpoint and comparing contract_etag satisfies the
// contract change detection requirement.
//
// The runtime block is informational only. Agents MUST NOT rely on
// runtime values for correctness or policy decisions.
#DiscoveryResponse: {
	version:       string
	service:       string
	description:   string

	// Opaque identifier for the current contract set.
	// Content-addressed: changes when any contract file changes.
	contract_etag: string

	// Resolved persona for the authenticated caller.
	// Derived from the bearer token, not a service default.
	persona: string

	contracts: {
		cue:            string // path or URL to CUE contract files
		openapi?:       string // path or URL to generated OpenAPI spec
		stdlib_version: #StdlibVersion
	}

	// Informational only. Do not use for policy decisions.
	runtime?: {
		active_flows?:   [...string]
		snapshot_count?: int
	}
}

// ─── EXECUTION ───────────────────────────────────────────────────────────────

// ExecuteRequest is the body of POST /execute.
// The executor validates contract_etag before any evaluation proceeds.
// Dry-run does not bypass etag validation.
#ExecuteRequest: {
	operation: string

	// Input matching the operation's declared input schema.
	input: {...}

	// If true: run steps 1-5 only. No side effects. No state transitions.
	// Returns fact_snapshot, verdicts, rules_matched, and a would_* outcome.
	// Executors SHOULD implement dry-run.
	// Agents SHOULD use dry-run before irreversible operations.
	dry_run?: bool | *false

	// Client-known contract version. If supplied, the executor MUST
	// compare against the active contract_etag and reject with HTTP 409
	// and CONTRACT_VERSION_MISMATCH if they differ.
	// Agents that cache contracts SHOULD include this field — it enables
	// the executor to detect races between contract re-fetch and invocation.
	contract_etag?: string
}

// ExecuteOutcome is the result of evaluation.
//   "executed"       — operation completed, side effects occurred
//   "denied"         — a deny verdict was produced, no side effects
//   "escalated"      — an escalate verdict was produced, no side effects
//   "required"       — a require verdict was produced, no side effects
//   "system_error"   — fact unavailable or executor error, no side effects
//   "dry_run"        — dry-run completed, no side effects
//   "would_execute"  — dry-run: would have executed
//   "would_deny"     — dry-run: would have been denied
//   "would_escalate" — dry-run: would have been escalated
//   "would_require"  — dry-run: would have required additional conditions
#ExecuteOutcome: "executed" | "denied" | "escalated" | "required" | "system_error" | "dry_run" | "would_execute" | "would_deny" | "would_escalate" | "would_require"

// ExecuteResponse is the response from POST /execute.
#ExecuteResponse: {
	outcome:  #ExecuteOutcome
	dry_run?: bool

	// Present on successful execution.
	output?: {...}

	// Present on deny, escalate, require, or system_error.
	error?: #ErrorEnvelope & {
		// FACT_UNAVAILABLE indicates a port fact could not be gathered.
		// Distinguishable from business-rule denials in both response
		// and audit record. Includes which fact failed and why.
		code?: string
		fact?: string  // present when code is FACT_UNAVAILABLE
		reason?: "timeout" | "unavailable" | "null" // present when FACT_UNAVAILABLE
	}

	// Present on dry-run. All facts used in evaluation.
	fact_snapshot?: {[name=string]: _}

	// All verdicts produced, in resolution order.
	verdicts?: [...{
		type:   #VerdictType
		code:   string
		reason: string
	}]

	// IDs of all rules that matched.
	rules_matched?: [...string]
}

// ─── AUDIT ───────────────────────────────────────────────────────────────────

// AuditRecord is produced by the executor for every invocation — live or dry-run.
// Audit generation occurs exclusively in the executor.
// Port adapters MUST NOT generate independent audit records for invocations.
// Elevation events produce a separate audit record referencing invocation_id.
#AuditRecord: {
	invocation_id:   string
	timestamp:       string // ISO 8601 UTC
	agent_id:        string
	persona:         string
	operation:       string
	input_payload:   {...}
	contract_version: string // active contract_etag at time of invocation
	executor_version: string
	fact_snapshot:   {[name=string]: _}
	verdicts:        [...{type: #VerdictType, code: string}]
	rules_matched:   [...string]
	outcome:         #ExecuteOutcome
	error_code?:     string
	duration_ms:     number
}
