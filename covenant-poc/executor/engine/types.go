package engine

// Contract holds the parsed domain contract extracted from CUE sources.
type Contract struct {
	Facts        map[string]FactDef
	DerivedFacts map[string]DerivedFactDef
	Rules        []RuleDef
	Operations   map[string]OperationDef
	Entities     map[string]EntityDef
}

type FactDef struct {
	Source    string // "input", "ctx", "port:<name>"
	Required  bool
	OnMissing string // "system_error" (default), "deny", "skip"
}

type DerivedFactDef struct {
	Derivation Derivation
}

type Derivation struct {
	Fn   string          `json:"fn"`
	Args []DerivationArg `json:"args"`
}

type DerivationArg struct {
	Fact  string `json:"fact,omitempty"`
	Op    string `json:"op,omitempty"`
	Value any    `json:"value,omitempty"`
}

type RuleDef struct {
	ID        string     `json:"id"`
	AppliesTo []string   `json:"applies_to"`
	When      Condition  `json:"when"`
	Verdict   VerdictDef `json:"verdict"`
}

type Condition struct {
	All         []Condition `json:"all,omitempty"`
	Any         []Condition `json:"any,omitempty"`
	Not         *Condition  `json:"not,omitempty"`
	Fact        string      `json:"fact,omitempty"`
	Equals      any         `json:"equals,omitempty"`
	GreaterThan any         `json:"greater_than,omitempty"`
	LessThan    any         `json:"less_than,omitempty"`
	In          []any       `json:"in,omitempty"`
}

type VerdictDef struct {
	Deny     *DenyVerdict     `json:"deny,omitempty"`
	Escalate *EscalateVerdict `json:"escalate,omitempty"`
	Require  *RequireVerdict  `json:"require,omitempty"`
	Flag     *FlagVerdict     `json:"flag,omitempty"`
}

type DenyVerdict struct {
	Code   string        `json:"code"`
	Reason string        `json:"reason"`
	Error  ErrorEnvelope `json:"error"`
}

type EscalateVerdict struct {
	Queue  string `json:"queue"`
	Reason string `json:"reason"`
}

type RequireVerdict struct {
	Conditions []string `json:"conditions"`
	Reason     string   `json:"reason"`
}

type FlagVerdict struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type ErrorEnvelope struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HttpStatus int    `json:"http_status"`
	Category   string `json:"category"`
	Retryable  bool   `json:"retryable"`
	Suggestion string `json:"suggestion,omitempty"`
}

type OperationDef struct {
	ConstrainedBy []string              `json:"constrained_by"`
	Transitions   []EntityTransitionRef `json:"transitions"`
}

type EntityTransitionRef struct {
	Entity string `json:"entity"`
	From   string `json:"from,omitempty"`
	To     string `json:"to"`
}

type EntityDef struct {
	States      []string     `json:"states"`
	Initial     string       `json:"initial"`
	Terminal    []string     `json:"terminal"`
	Transitions []Transition `json:"transitions"`
}

type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
}

// Request is the payload sent to POST /execute.
type Request struct {
	Operation    string         `json:"operation"`
	Input        map[string]any `json:"input"`
	DryRun       bool           `json:"dry_run"`
	ContractETag string         `json:"contract_etag,omitempty"`
}

// Response is returned from POST /execute.
type Response struct {
	Outcome      string         `json:"outcome"`
	Output       map[string]any `json:"output,omitempty"`
	Error        *ErrorEnvelope `json:"error,omitempty"`
	Verdicts     []Verdict      `json:"verdicts,omitempty"`
	FactSnapshot map[string]any `json:"fact_snapshot,omitempty"`
	DryRun       bool           `json:"dry_run,omitempty"`
}

// Verdict is a resolved verdict from rule evaluation.
type Verdict struct {
	Type   string         `json:"type"` // deny, escalate, require, flag
	Code   string         `json:"code,omitempty"`
	Reason string         `json:"reason,omitempty"`
	Error  *ErrorEnvelope `json:"error,omitempty"`
	Queue  string         `json:"queue,omitempty"`
}
