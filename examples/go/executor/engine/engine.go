package engine

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
)

// Engine interprets a loaded Contract and evaluates operations against it.
type Engine struct {
	mu           sync.RWMutex
	contract     *Contract
	contractETag string
	ports        PortRegistry
}

// PortRegistry provides access to port adapters by name.
type PortRegistry interface {
	Get(ctx context.Context, port, fact string, input map[string]any) (any, error)
	Execute(ctx context.Context, port, operation string, input map[string]any) (map[string]any, error)
}

func NewEngine(ports PortRegistry) *Engine {
	return &Engine{ports: ports}
}

func (e *Engine) LoadContract(c *Contract, etag string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.contract = c
	e.contractETag = etag
}

func (e *Engine) ETag() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.contractETag
}

// Evaluate runs the Section 11 evaluation algorithm for the given request.
func (e *Engine) Evaluate(ctx context.Context, req *Request) (*Response, error) {
	e.mu.RLock()
	contract := e.contract
	etag := e.contractETag
	e.mu.RUnlock()

	if contract == nil {
		return nil, fmt.Errorf("no contract loaded")
	}

	// Validate contract ETag if supplied.
	if req.ContractETag != "" && req.ContractETag != etag {
		return &Response{
			Outcome: "system_error",
			Error: &ErrorEnvelope{
				Code:       "CONTRACT_VERSION_MISMATCH",
				Message:    "Client contract version is stale — re-fetch contracts and retry",
				HttpStatus: 409,
				Category:   "system",
				Retryable:  true,
			},
		}, nil
	}

	op, ok := contract.Operations[req.Operation]
	if !ok {
		return nil, fmt.Errorf("unknown operation: %s", req.Operation)
	}

	// Step 1: Gather base facts.
	facts, err := e.gatherFacts(ctx, contract, req.Operation, req.Input)
	if err != nil {
		if fe, ok := err.(*factError); ok {
			return &Response{
				Outcome: fe.outcome,
				Error: &ErrorEnvelope{
					Code:       "FACT_UNAVAILABLE",
					Message:    fmt.Sprintf("fact %q unavailable: %s", fe.fact, fe.reason),
					HttpStatus: 503,
					Category:   "system",
					Retryable:  true,
				},
			}, nil
		}
		return nil, err
	}

	// Step 2: Derive computed facts.
	if err := e.deriveFacts(contract, facts); err != nil {
		return nil, fmt.Errorf("derive facts: %w", err)
	}

	// Step 3: Validate entity state (simplified — transitions declared on operation).
	// For this POC we skip state machine validation since we don't track live state.

	// Step 4: Evaluate rules.
	verdicts := e.evaluateRules(contract, req.Operation, facts)

	// Step 5: Apply verdict.
	final := resolveVerdicts(verdicts)

	if req.DryRun {
		return &Response{
			DryRun:       true,
			Outcome:      dryRunOutcome(final),
			Verdicts:     verdicts,
			FactSnapshot: facts.Snapshot(),
		}, nil
	}

	if final != nil && final.Type == "deny" {
		return &Response{
			Outcome:  "denied",
			Error:    final.Error,
			Verdicts: verdicts,
		}, nil
	}

	if final != nil && final.Type == "escalate" {
		return &Response{
			Outcome:  "escalated",
			Verdicts: verdicts,
		}, nil
	}

	// Step 6: Execute — side effects happen here only.
	result, err := e.ports.Execute(ctx, operationPort(op), req.Operation, req.Input)
	if err != nil {
		return &Response{
			Outcome: "system_error",
			Error: &ErrorEnvelope{
				Code:       "EXECUTION_FAILED",
				Message:    err.Error(),
				HttpStatus: 500,
				Category:   "system",
				Retryable:  true,
			},
		}, nil
	}

	// Step 7: Transition entity state (recorded in port adapter for this POC).

	resp := &Response{
		Outcome: "executed",
		Output:  result,
	}
	if len(verdicts) > 0 {
		resp.Verdicts = verdicts // include any flags
	}
	return resp, nil
}

// operationPort returns the primary port for executing an operation.
// In this POC, ProcessPayment is handled by invoiceRepo; GetInvoice also by invoiceRepo.
func operationPort(_ OperationDef) string {
	return "invoiceRepo"
}

// gatherFacts collects the base facts needed by the operation's rules.
// Only facts relevant to the operation are validated as required.
// Port facts are fetched in parallel.
func (e *Engine) gatherFacts(ctx context.Context, c *Contract, operation string, input map[string]any) (*FactSet, error) {
	facts := NewFactSet()

	needed := neededBaseFacts(c, operation)

	type portResult struct {
		name string
		val  any
		err  error
		def  FactDef
	}

	ch := make(chan portResult, len(needed))
	var wg sync.WaitGroup

	for name := range needed {
		def, ok := c.Facts[name]
		if !ok {
			continue
		}
		switch {
		case def.Source == "input":
			if val, ok := input[name]; ok {
				facts.Set(name, val)
			} else if def.Required {
				return nil, fmt.Errorf("required input fact %q missing from request", name)
			}
		case def.Source == "ctx":
			if name == "user.roles" {
				facts.Set(name, []string{"customer"})
			}
		case strings.HasPrefix(def.Source, "port:"):
			wg.Add(1)
			go func(n string, d FactDef) {
				defer wg.Done()
				val, err := e.ports.Get(ctx, portName(d.Source), n, input)
				ch <- portResult{name: n, val: val, err: err, def: d}
			}(name, def)
		}
	}

	go func() { wg.Wait(); close(ch) }()

	for r := range ch {
		if r.err != nil {
			switch r.def.OnMissing {
			case "deny":
				return nil, &factError{fact: r.name, reason: r.err.Error(), outcome: "denied"}
			case "skip":
				// Fact absent — conditions referencing it evaluate to false.
			default: // "system_error"
				return nil, &factError{fact: r.name, reason: r.err.Error(), outcome: "system_error"}
			}
			continue
		}
		facts.Set(r.name, r.val)
	}

	return facts, nil
}

// neededBaseFacts returns the set of base fact names (all sources) required by
// the rules that constrain the given operation.
// Dotted paths like "payment.amount.value" are resolved to their base fact "payment.amount".
func neededBaseFacts(c *Contract, operation string) map[string]bool {
	needed := map[string]bool{}
	derivedVisited := map[string]bool{}

	var addPath func(path string)
	addPath = func(path string) {
		// Exact base fact.
		if _, ok := c.Facts[path]; ok {
			needed[path] = true
			return
		}
		// Derived fact — recurse into its arg dependencies.
		if df, ok := c.DerivedFacts[path]; ok {
			if derivedVisited[path] {
				return
			}
			derivedVisited[path] = true
			for _, arg := range df.Derivation.Args {
				if arg.Fact != "" {
					addPath(arg.Fact)
				}
			}
			return
		}
		// Dotted path into a fact — find the longest matching prefix.
		parts := strings.Split(path, ".")
		for i := len(parts) - 1; i > 0; i-- {
			prefix := strings.Join(parts[:i], ".")
			if _, ok := c.Facts[prefix]; ok {
				needed[prefix] = true
				return
			}
			if _, ok := c.DerivedFacts[prefix]; ok {
				addPath(prefix)
				return
			}
		}
	}

	op, ok := c.Operations[operation]
	if !ok {
		return needed
	}
	for _, ruleID := range op.ConstrainedBy {
		for i := range c.Rules {
			if c.Rules[i].ID == ruleID {
				collectFromCondition(c.Rules[i].When, addPath)
			}
		}
	}
	return needed
}

func collectFromCondition(cond Condition, collect func(string)) {
	if cond.Fact != "" {
		collect(cond.Fact)
	}
	for _, sub := range cond.All {
		collectFromCondition(sub, collect)
	}
	for _, sub := range cond.Any {
		collectFromCondition(sub, collect)
	}
	if cond.Not != nil {
		collectFromCondition(*cond.Not, collect)
	}
}

// deriveFacts evaluates derived facts in topological order.
func (e *Engine) deriveFacts(c *Contract, facts *FactSet) error {
	order := topoSort(c.DerivedFacts)
	for _, name := range order {
		df := c.DerivedFacts[name]
		val, err := evalDerivation(df.Derivation, facts)
		if err != nil {
			return fmt.Errorf("derive %q: %w", name, err)
		}
		facts.Set(name, val)
	}
	return nil
}

// topoSort returns derived fact names in dependency order (dependencies first).
func topoSort(dfs map[string]DerivedFactDef) []string {
	visited := map[string]bool{}
	var order []string

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		df, ok := dfs[name]
		if !ok {
			return
		}
		for _, arg := range df.Derivation.Args {
			if arg.Fact != "" {
				visit(arg.Fact)
			}
		}
		order = append(order, name)
	}

	for name := range dfs {
		visit(name)
	}
	return order
}

// evalDerivation evaluates a single derivation against the fact set.
func evalDerivation(d Derivation, facts *FactSet) (any, error) {
	getArg := func(arg DerivationArg) (any, bool) {
		if arg.Fact != "" {
			return facts.GetPath(arg.Fact)
		}
		return arg.Value, arg.Value != nil
	}

	switch d.Fn {
	case "greater_than":
		if len(d.Args) < 2 {
			return false, nil
		}
		a, _ := getArg(d.Args[0])
		b, _ := getArg(d.Args[1])
		fa, oka := toFloat(a)
		fb, okb := toFloat(b)
		if oka && okb {
			return fa > fb, nil
		}
		return false, nil

	case "greater_or_equal":
		if len(d.Args) < 2 {
			return false, nil
		}
		a, _ := getArg(d.Args[0])
		b, _ := getArg(d.Args[1])
		fa, oka := toFloat(a)
		fb, okb := toFloat(b)
		if oka && okb {
			return fa >= fb, nil
		}
		return false, nil

	case "less_than":
		if len(d.Args) < 2 {
			return false, nil
		}
		a, _ := getArg(d.Args[0])
		b, _ := getArg(d.Args[1])
		fa, oka := toFloat(a)
		fb, okb := toFloat(b)
		if oka && okb {
			return fa < fb, nil
		}
		return false, nil

	case "equals":
		if len(d.Args) < 2 {
			return false, nil
		}
		a, _ := getArg(d.Args[0])
		b, _ := getArg(d.Args[1])
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b), nil

	case "and":
		for _, arg := range d.Args {
			// Args may include {fact, op, value} for inline conditions.
			if arg.Fact != "" && arg.Op != "" {
				factVal, _ := facts.GetPath(arg.Fact)
				result := applyOp(arg.Op, factVal, arg.Value)
				if !result {
					return false, nil
				}
			} else {
				v, _ := getArg(arg)
				if b, ok := v.(bool); ok && !b {
					return false, nil
				}
			}
		}
		return true, nil

	case "or":
		for _, arg := range d.Args {
			v, _ := getArg(arg)
			if b, ok := v.(bool); ok && b {
				return true, nil
			}
		}
		return false, nil

	case "not":
		if len(d.Args) == 0 {
			return true, nil
		}
		v, _ := getArg(d.Args[0])
		if b, ok := v.(bool); ok {
			return !b, nil
		}
		return false, nil

	default:
		return nil, fmt.Errorf("unknown derivation function: %s", d.Fn)
	}
}

// evaluateRules returns all matching verdicts for the given operation.
func (e *Engine) evaluateRules(c *Contract, operation string, facts *FactSet) []Verdict {
	var verdicts []Verdict

	op := c.Operations[operation]
	ruleSet := map[string]bool{}
	for _, id := range op.ConstrainedBy {
		ruleSet[id] = true
	}

	for _, rule := range c.Rules {
		if !ruleSet[rule.ID] {
			continue
		}
		if !evalCondition(rule.When, facts) {
			continue
		}
		v := rule.Verdict
		switch {
		case v.Deny != nil:
			e := v.Deny.Error
			verdicts = append(verdicts, Verdict{
				Type:   "deny",
				Code:   v.Deny.Code,
				Reason: v.Deny.Reason,
				Error:  &e,
			})
		case v.Escalate != nil:
			verdicts = append(verdicts, Verdict{
				Type:   "escalate",
				Reason: v.Escalate.Reason,
				Queue:  v.Escalate.Queue,
			})
		case v.Require != nil:
			verdicts = append(verdicts, Verdict{
				Type:   "require",
				Reason: v.Require.Reason,
			})
		case v.Flag != nil:
			verdicts = append(verdicts, Verdict{
				Type:   "flag",
				Code:   v.Flag.Code,
				Reason: v.Flag.Reason,
			})
		}
	}

	return verdicts
}

// evalCondition evaluates a condition tree against the fact set.
func evalCondition(cond Condition, facts *FactSet) bool {
	switch {
	case len(cond.All) > 0:
		for _, sub := range cond.All {
			if !evalCondition(sub, facts) {
				return false
			}
		}
		return true

	case len(cond.Any) > 0:
		for _, sub := range cond.Any {
			if evalCondition(sub, facts) {
				return true
			}
		}
		return false

	case cond.Not != nil:
		return !evalCondition(*cond.Not, facts)

	case cond.Fact != "":
		val, _ := facts.GetPath(cond.Fact)
		switch {
		case cond.Equals != nil:
			return applyOp("equals", val, cond.Equals)
		case cond.GreaterThan != nil:
			return applyOp("greater_than", val, cond.GreaterThan)
		case cond.LessThan != nil:
			return applyOp("less_than", val, cond.LessThan)
		case len(cond.In) > 0:
			for _, v := range cond.In {
				if applyOp("equals", val, v) {
					return true
				}
			}
			return false
		}
	}
	return true
}

func applyOp(op string, left, right any) bool {
	switch op {
	case "equals":
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	case "greater_than":
		fl, okl := toFloat(left)
		fr, okr := toFloat(right)
		return okl && okr && fl > fr
	case "less_than":
		fl, okl := toFloat(left)
		fr, okr := toFloat(right)
		return okl && okr && fl < fr
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	}
	return 0, false
}

// resolveVerdicts returns the highest-priority verdict (deny > escalate > require > flag).
func resolveVerdicts(verdicts []Verdict) *Verdict {
	priority := map[string]int{"deny": 4, "escalate": 3, "require": 2, "flag": 1}
	var best *Verdict
	for i := range verdicts {
		v := &verdicts[i]
		if best == nil || priority[v.Type] > priority[best.Type] {
			best = v
		}
	}
	return best
}

func dryRunOutcome(v *Verdict) string {
	if v == nil {
		return "would_execute"
	}
	switch v.Type {
	case "deny":
		return "would_deny"
	case "escalate":
		return "would_escalate"
	case "require":
		return "would_require"
	default:
		return "would_execute_with_flags"
	}
}

// randID generates a short random alphanumeric ID.
func randID(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

type factError struct {
	fact    string
	reason  string
	outcome string
}

func (e *factError) Error() string {
	return fmt.Sprintf("fact %q: %s", e.fact, e.reason)
}
