package engine

import (
	"context"
	"fmt"
	"testing"
)

// mockPorts implements PortRegistry for tests.
type mockPorts struct {
	getFunc     func(ctx context.Context, port, fact string, input map[string]any) (any, error)
	executeFunc func(ctx context.Context, port, operation string, input map[string]any) (map[string]any, error)
}

func (m *mockPorts) Get(ctx context.Context, port, fact string, input map[string]any) (any, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, port, fact, input)
	}
	return nil, nil
}

func (m *mockPorts) Execute(ctx context.Context, port, operation string, input map[string]any) (map[string]any, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, port, operation, input)
	}
	return map[string]any{}, nil
}

// --- evalCondition ---

func TestEvalCondition_equalsMatchesStringFact(t *testing.T) {
	fs := NewFactSet()
	fs.Set("status", "active")
	if !evalCondition(Condition{Fact: "status", Equals: "active"}, fs) {
		t.Fatal("expected condition to match")
	}
}

func TestEvalCondition_equalsNoMatchReturnsFalse(t *testing.T) {
	fs := NewFactSet()
	fs.Set("status", "inactive")
	if evalCondition(Condition{Fact: "status", Equals: "active"}, fs) {
		t.Fatal("expected condition not to match")
	}
}

func TestEvalCondition_greaterThanNumeric(t *testing.T) {
	fs := NewFactSet()
	fs.Set("amount", 1000.0)
	if !evalCondition(Condition{Fact: "amount", GreaterThan: 500.0}, fs) {
		t.Fatal("expected 1000 > 500 to be true")
	}
}

func TestEvalCondition_greaterThanNotMetReturnsFalse(t *testing.T) {
	fs := NewFactSet()
	fs.Set("amount", 100.0)
	if evalCondition(Condition{Fact: "amount", GreaterThan: 500.0}, fs) {
		t.Fatal("expected 100 > 500 to be false")
	}
}

func TestEvalCondition_lessThanNumeric(t *testing.T) {
	fs := NewFactSet()
	fs.Set("score", 30.0)
	if !evalCondition(Condition{Fact: "score", LessThan: 50.0}, fs) {
		t.Fatal("expected 30 < 50 to be true")
	}
}

func TestEvalCondition_inMatchesOneOf(t *testing.T) {
	fs := NewFactSet()
	fs.Set("tier", "gold")
	cond := Condition{Fact: "tier", In: []any{"silver", "gold", "platinum"}}
	if !evalCondition(cond, fs) {
		t.Fatal("expected 'gold' to be in the list")
	}
}

func TestEvalCondition_inNoMatchReturnsFalse(t *testing.T) {
	fs := NewFactSet()
	fs.Set("tier", "bronze")
	cond := Condition{Fact: "tier", In: []any{"silver", "gold"}}
	if evalCondition(cond, fs) {
		t.Fatal("expected 'bronze' not to be in the list")
	}
}

func TestEvalCondition_allRequiresEverySubcondition(t *testing.T) {
	fs := NewFactSet()
	fs.Set("a", "x")
	fs.Set("b", "y")
	cond := Condition{All: []Condition{
		{Fact: "a", Equals: "x"},
		{Fact: "b", Equals: "y"},
	}}
	if !evalCondition(cond, fs) {
		t.Fatal("expected all to pass")
	}
}

func TestEvalCondition_allFailsIfOneFails(t *testing.T) {
	fs := NewFactSet()
	fs.Set("a", "x")
	fs.Set("b", "wrong")
	cond := Condition{All: []Condition{
		{Fact: "a", Equals: "x"},
		{Fact: "b", Equals: "y"},
	}}
	if evalCondition(cond, fs) {
		t.Fatal("expected all to fail when one subcondition fails")
	}
}

func TestEvalCondition_anyPassesIfOneMatches(t *testing.T) {
	fs := NewFactSet()
	fs.Set("a", "no")
	fs.Set("b", "yes")
	cond := Condition{Any: []Condition{
		{Fact: "a", Equals: "yes"},
		{Fact: "b", Equals: "yes"},
	}}
	if !evalCondition(cond, fs) {
		t.Fatal("expected any to pass when one subcondition matches")
	}
}

func TestEvalCondition_anyFailsIfNoneMatch(t *testing.T) {
	fs := NewFactSet()
	fs.Set("a", "no")
	fs.Set("b", "no")
	cond := Condition{Any: []Condition{
		{Fact: "a", Equals: "yes"},
		{Fact: "b", Equals: "yes"},
	}}
	if evalCondition(cond, fs) {
		t.Fatal("expected any to fail when no subcondition matches")
	}
}

func TestEvalCondition_notNegatesFalseToTrue(t *testing.T) {
	fs := NewFactSet()
	fs.Set("blocked", "true")
	// inner checks blocked == "false" → false; not(false) → true
	inner := Condition{Fact: "blocked", Equals: "false"}
	if !evalCondition(Condition{Not: &inner}, fs) {
		t.Fatal("expected not(false) to be true")
	}
}

func TestEvalCondition_notNegatesTrueToFalse(t *testing.T) {
	fs := NewFactSet()
	fs.Set("active", "yes")
	inner := Condition{Fact: "active", Equals: "yes"}
	if evalCondition(Condition{Not: &inner}, fs) {
		t.Fatal("expected not(true) to be false")
	}
}

func TestEvalCondition_missingFactReturnsFalseForEquals(t *testing.T) {
	fs := NewFactSet()
	if evalCondition(Condition{Fact: "nonexistent", Equals: "something"}, fs) {
		t.Fatal("expected missing fact not to match equals condition")
	}
}

// --- rule evaluation ---

func makeSimpleContract(ruleID string, verdict VerdictDef, cond Condition) *Contract {
	return &Contract{
		Facts: map[string]FactDef{
			"customer.status": {Source: "input", Required: false},
		},
		DerivedFacts: map[string]DerivedFactDef{},
		Rules: []RuleDef{
			{ID: ruleID, When: cond, Verdict: verdict},
		},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{ruleID}},
		},
		Entities: map[string]EntityDef{},
	}
}

func TestEvaluateRules_denyVerdictWhenConditionMatches(t *testing.T) {
	e := NewEngine(&mockPorts{})
	contract := makeSimpleContract("r1",
		VerdictDef{Deny: &DenyVerdict{
			Code:   "BLOCKED",
			Reason: "customer blocked",
			Error:  ErrorEnvelope{Code: "BLOCKED", Message: "blocked", HttpStatus: 403},
		}},
		Condition{Fact: "customer.status", Equals: "blocked"},
	)
	fs := NewFactSet()
	fs.Set("customer.status", "blocked")

	verdicts := e.evaluateRules(contract, "testOp", fs)

	if len(verdicts) != 1 {
		t.Fatalf("expected 1 verdict, got %d", len(verdicts))
	}
	if verdicts[0].Type != "deny" {
		t.Fatalf("expected deny verdict, got %s", verdicts[0].Type)
	}
	if verdicts[0].Code != "BLOCKED" {
		t.Fatalf("expected code BLOCKED, got %s", verdicts[0].Code)
	}
}

func TestEvaluateRules_flagVerdictWhenConditionMatches(t *testing.T) {
	e := NewEngine(&mockPorts{})
	contract := makeSimpleContract("r2",
		VerdictDef{Flag: &FlagVerdict{Code: "HIGH_VALUE", Reason: "high value transaction"}},
		Condition{Fact: "amount", GreaterThan: 1000.0},
	)
	fs := NewFactSet()
	fs.Set("amount", 2000.0)

	verdicts := e.evaluateRules(contract, "testOp", fs)

	if len(verdicts) != 1 || verdicts[0].Type != "flag" {
		t.Fatalf("expected flag verdict, got %+v", verdicts)
	}
	if verdicts[0].Code != "HIGH_VALUE" {
		t.Fatalf("expected HIGH_VALUE, got %s", verdicts[0].Code)
	}
}

func TestEvaluateRules_escalateVerdictWhenConditionMatches(t *testing.T) {
	e := NewEngine(&mockPorts{})
	contract := makeSimpleContract("r3",
		VerdictDef{Escalate: &EscalateVerdict{Queue: "fraud-review", Reason: "suspicious"}},
		Condition{Fact: "risk.score", GreaterThan: 90.0},
	)
	fs := NewFactSet()
	fs.Set("risk.score", 95.0)

	verdicts := e.evaluateRules(contract, "testOp", fs)

	if len(verdicts) != 1 || verdicts[0].Type != "escalate" {
		t.Fatalf("expected escalate verdict, got %+v", verdicts)
	}
	if verdicts[0].Queue != "fraud-review" {
		t.Fatalf("expected fraud-review queue, got %s", verdicts[0].Queue)
	}
}

func TestEvaluateRules_noVerdictWhenConditionDoesNotMatch(t *testing.T) {
	e := NewEngine(&mockPorts{})
	contract := makeSimpleContract("r4",
		VerdictDef{Deny: &DenyVerdict{Code: "DENIED"}},
		Condition{Fact: "customer.status", Equals: "blocked"},
	)
	fs := NewFactSet()
	fs.Set("customer.status", "active")

	verdicts := e.evaluateRules(contract, "testOp", fs)

	if len(verdicts) != 0 {
		t.Fatalf("expected no verdicts, got %+v", verdicts)
	}
}

func TestEvaluateRules_ruleNotInOperationConstraintsIsSkipped(t *testing.T) {
	e := NewEngine(&mockPorts{})
	contract := &Contract{
		DerivedFacts: map[string]DerivedFactDef{},
		Rules: []RuleDef{
			{
				ID:   "unrelated-rule",
				When: Condition{Fact: "x", Equals: "y"},
				Verdict: VerdictDef{Deny: &DenyVerdict{Code: "DENIED"}},
			},
		},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{}}, // does NOT reference "unrelated-rule"
		},
		Entities: map[string]EntityDef{},
	}
	fs := NewFactSet()
	fs.Set("x", "y")

	verdicts := e.evaluateRules(contract, "testOp", fs)

	if len(verdicts) != 0 {
		t.Fatalf("expected rule not in ConstrainedBy to be skipped, got %+v", verdicts)
	}
}

// --- topoSort ---

func TestTopoSort_independentFactsAllPresent(t *testing.T) {
	dfs := map[string]DerivedFactDef{
		"a": {Derivation: Derivation{Fn: "equals", Args: []DerivationArg{{Value: true}}}},
		"b": {Derivation: Derivation{Fn: "equals", Args: []DerivationArg{{Value: false}}}},
	}
	order := topoSort(dfs)
	if len(order) != 2 {
		t.Fatalf("expected 2 items, got %d: %v", len(order), order)
	}
}

func TestTopoSort_dependencyComesBeforeDependent(t *testing.T) {
	// "b" depends on "a" — "a" must appear before "b" in the order.
	dfs := map[string]DerivedFactDef{
		"a": {Derivation: Derivation{
			Fn:   "greater_than",
			Args: []DerivationArg{{Value: 100.0}, {Value: 50.0}},
		}},
		"b": {Derivation: Derivation{
			Fn:   "not",
			Args: []DerivationArg{{Fact: "a"}},
		}},
	}
	order := topoSort(dfs)
	if len(order) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(order), order)
	}
	idxA, idxB := -1, -1
	for i, n := range order {
		if n == "a" {
			idxA = i
		}
		if n == "b" {
			idxB = i
		}
	}
	if idxA == -1 || idxB == -1 {
		t.Fatalf("missing names in order: %v", order)
	}
	if idxA > idxB {
		t.Fatalf("expected 'a' before 'b', got order %v", order)
	}
}

// --- evalDerivation ---

func TestEvalDerivation_greaterThanTrueWhenLeftExceedsRight(t *testing.T) {
	fs := NewFactSet()
	fs.Set("x", 10.0)
	d := Derivation{Fn: "greater_than", Args: []DerivationArg{
		{Fact: "x"}, {Value: 5.0},
	}}
	got, err := evalDerivation(d, fs)
	if err != nil {
		t.Fatal(err)
	}
	if got != true {
		t.Fatalf("expected true, got %v", got)
	}
}

func TestEvalDerivation_greaterThanFalseWhenLeftBelowRight(t *testing.T) {
	fs := NewFactSet()
	fs.Set("x", 3.0)
	d := Derivation{Fn: "greater_than", Args: []DerivationArg{
		{Fact: "x"}, {Value: 5.0},
	}}
	got, _ := evalDerivation(d, fs)
	if got != false {
		t.Fatalf("expected false, got %v", got)
	}
}

func TestEvalDerivation_greaterOrEqualTrueForEqual(t *testing.T) {
	fs := NewFactSet()
	fs.Set("x", 5.0)
	d := Derivation{Fn: "greater_or_equal", Args: []DerivationArg{
		{Fact: "x"}, {Value: 5.0},
	}}
	got, _ := evalDerivation(d, fs)
	if got != true {
		t.Fatalf("expected true for 5 >= 5, got %v", got)
	}
}

func TestEvalDerivation_greaterOrEqualFalseWhenLess(t *testing.T) {
	fs := NewFactSet()
	fs.Set("x", 4.0)
	d := Derivation{Fn: "greater_or_equal", Args: []DerivationArg{
		{Fact: "x"}, {Value: 5.0},
	}}
	got, _ := evalDerivation(d, fs)
	if got != false {
		t.Fatalf("expected false for 4 >= 5, got %v", got)
	}
}

func TestEvalDerivation_lessThanTrue(t *testing.T) {
	fs := NewFactSet()
	fs.Set("x", 2.0)
	d := Derivation{Fn: "less_than", Args: []DerivationArg{
		{Fact: "x"}, {Value: 5.0},
	}}
	got, _ := evalDerivation(d, fs)
	if got != true {
		t.Fatalf("expected true for 2 < 5, got %v", got)
	}
}

func TestEvalDerivation_equalsStringsMatch(t *testing.T) {
	fs := NewFactSet()
	fs.Set("s", "hello")
	d := Derivation{Fn: "equals", Args: []DerivationArg{
		{Fact: "s"}, {Value: "hello"},
	}}
	got, _ := evalDerivation(d, fs)
	if got != true {
		t.Fatalf("expected true for string equality, got %v", got)
	}
}

func TestEvalDerivation_equalsStringsMismatch(t *testing.T) {
	fs := NewFactSet()
	fs.Set("s", "hello")
	d := Derivation{Fn: "equals", Args: []DerivationArg{
		{Fact: "s"}, {Value: "world"},
	}}
	got, _ := evalDerivation(d, fs)
	if got != false {
		t.Fatalf("expected false for string mismatch, got %v", got)
	}
}

func TestEvalDerivation_andReturnsTrueWhenAllTrue(t *testing.T) {
	fs := NewFactSet()
	fs.Set("p", true)
	fs.Set("q", true)
	d := Derivation{Fn: "and", Args: []DerivationArg{
		{Fact: "p"}, {Fact: "q"},
	}}
	got, _ := evalDerivation(d, fs)
	if got != true {
		t.Fatalf("expected true, got %v", got)
	}
}

func TestEvalDerivation_andReturnsFalseWhenOneFalse(t *testing.T) {
	fs := NewFactSet()
	fs.Set("p", true)
	fs.Set("q", false)
	d := Derivation{Fn: "and", Args: []DerivationArg{
		{Fact: "p"}, {Fact: "q"},
	}}
	got, _ := evalDerivation(d, fs)
	if got != false {
		t.Fatalf("expected false, got %v", got)
	}
}

func TestEvalDerivation_orReturnsTrueWhenOneTrue(t *testing.T) {
	fs := NewFactSet()
	fs.Set("p", false)
	fs.Set("q", true)
	d := Derivation{Fn: "or", Args: []DerivationArg{
		{Fact: "p"}, {Fact: "q"},
	}}
	got, _ := evalDerivation(d, fs)
	if got != true {
		t.Fatalf("expected true, got %v", got)
	}
}

func TestEvalDerivation_orReturnsFalseWhenNoneTrue(t *testing.T) {
	fs := NewFactSet()
	fs.Set("p", false)
	fs.Set("q", false)
	d := Derivation{Fn: "or", Args: []DerivationArg{
		{Fact: "p"}, {Fact: "q"},
	}}
	got, _ := evalDerivation(d, fs)
	if got != false {
		t.Fatalf("expected false, got %v", got)
	}
}

func TestEvalDerivation_notNegatesBool(t *testing.T) {
	fs := NewFactSet()
	fs.Set("flag", false)
	d := Derivation{Fn: "not", Args: []DerivationArg{{Fact: "flag"}}}
	got, _ := evalDerivation(d, fs)
	if got != true {
		t.Fatalf("expected not(false)=true, got %v", got)
	}
}

func TestEvalDerivation_unknownFnReturnsError(t *testing.T) {
	fs := NewFactSet()
	_, err := evalDerivation(Derivation{Fn: "bogus"}, fs)
	if err == nil {
		t.Fatal("expected error for unknown derivation function")
	}
}

// --- deriveFacts integration ---

func TestDeriveFacts_evaluatesChainInTopologicalOrder(t *testing.T) {
	e := NewEngine(&mockPorts{})
	contract := &Contract{
		DerivedFacts: map[string]DerivedFactDef{
			// "should_flag" depends on "is_high_value", so "is_high_value" must be evaluated first.
			"is_high_value": {Derivation: Derivation{
				Fn:   "greater_than",
				Args: []DerivationArg{{Fact: "amount"}, {Value: 500.0}},
			}},
			"should_flag": {Derivation: Derivation{
				Fn:   "not",
				Args: []DerivationArg{{Fact: "is_high_value"}},
			}},
		},
	}
	fs := NewFactSet()
	fs.Set("amount", 1000.0)

	if err := e.deriveFacts(contract, fs); err != nil {
		t.Fatal(err)
	}

	isHighVal, ok := fs.Get("is_high_value")
	if !ok || isHighVal != true {
		t.Fatalf("expected is_high_value=true, got %v (found=%v)", isHighVal, ok)
	}
	shouldFlag, ok := fs.Get("should_flag")
	if !ok || shouldFlag != false {
		t.Fatalf("expected should_flag=false (not of true), got %v", shouldFlag)
	}
}

// --- Engine.Evaluate ---

func makeMinimalContract() *Contract {
	return &Contract{
		Facts:        map[string]FactDef{},
		DerivedFacts: map[string]DerivedFactDef{},
		Rules:        []RuleDef{},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{}},
		},
		Entities: map[string]EntityDef{},
	}
}

func TestEngine_Evaluate_happyPathReturnsExecuted(t *testing.T) {
	ports := &mockPorts{
		executeFunc: func(_ context.Context, _, _ string, _ map[string]any) (map[string]any, error) {
			return map[string]any{"result": "ok"}, nil
		},
	}
	eng := NewEngine(ports)
	eng.LoadContract(makeMinimalContract(), "etag-1")

	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation: "testOp",
		Input:     map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "executed" {
		t.Fatalf("expected executed, got %s", resp.Outcome)
	}
	if resp.Output["result"] != "ok" {
		t.Fatalf("expected output result=ok, got %v", resp.Output)
	}
}

func TestEngine_Evaluate_denyReturnsOutcomeDenied(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	contract := &Contract{
		Facts: map[string]FactDef{
			"customer.status": {Source: "input", Required: false},
		},
		DerivedFacts: map[string]DerivedFactDef{},
		Rules: []RuleDef{
			{
				ID:   "block-rule",
				When: Condition{Fact: "customer.status", Equals: "blocked"},
				Verdict: VerdictDef{Deny: &DenyVerdict{
					Code:   "CUSTOMER_BLOCKED",
					Reason: "blocked",
					Error:  ErrorEnvelope{Code: "CUSTOMER_BLOCKED", Message: "blocked", HttpStatus: 403},
				}},
			},
		},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{"block-rule"}},
		},
		Entities: map[string]EntityDef{},
	}
	eng.LoadContract(contract, "etag-1")

	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation: "testOp",
		Input:     map[string]any{"customer.status": "blocked"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "denied" {
		t.Fatalf("expected denied, got %s", resp.Outcome)
	}
	if resp.Error == nil || resp.Error.Code != "CUSTOMER_BLOCKED" {
		t.Fatalf("expected CUSTOMER_BLOCKED error, got %+v", resp.Error)
	}
}

func TestEngine_Evaluate_escalateReturnsOutcomeEscalated(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	contract := &Contract{
		Facts:        map[string]FactDef{},
		DerivedFacts: map[string]DerivedFactDef{},
		Rules: []RuleDef{
			{
				ID:      "escalate-rule",
				When:    Condition{Fact: "risk", GreaterThan: 90.0},
				Verdict: VerdictDef{Escalate: &EscalateVerdict{Queue: "review", Reason: "risky"}},
			},
		},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{"escalate-rule"}},
		},
		Entities: map[string]EntityDef{},
	}
	eng.LoadContract(contract, "etag-1")

	// Pre-set the fact directly via a port mock returning it — or use a port fact.
	// Simplest: put it as an input fact declared with source input.
	contract.Facts["risk"] = FactDef{Source: "input", Required: false}

	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation: "testOp",
		Input:     map[string]any{"risk": 95.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "escalated" {
		t.Fatalf("expected escalated, got %s", resp.Outcome)
	}
}

func TestEngine_Evaluate_dryRunWouldExecute(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	eng.LoadContract(makeMinimalContract(), "etag-1")

	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation: "testOp",
		Input:     map[string]any{},
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.DryRun {
		t.Fatal("expected DryRun=true in response")
	}
	if resp.Outcome != "would_execute" {
		t.Fatalf("expected would_execute, got %s", resp.Outcome)
	}
	if resp.FactSnapshot == nil {
		t.Fatal("expected fact snapshot in dry-run response")
	}
}

func TestEngine_Evaluate_dryRunWouldDeny(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	contract := &Contract{
		Facts: map[string]FactDef{
			"customer.status": {Source: "input", Required: false},
		},
		DerivedFacts: map[string]DerivedFactDef{},
		Rules: []RuleDef{
			{
				ID:   "block-rule",
				When: Condition{Fact: "customer.status", Equals: "blocked"},
				Verdict: VerdictDef{Deny: &DenyVerdict{
					Code:   "BLOCKED",
					Reason: "blocked",
					Error:  ErrorEnvelope{Code: "BLOCKED", Message: "blocked", HttpStatus: 403},
				}},
			},
		},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{"block-rule"}},
		},
		Entities: map[string]EntityDef{},
	}
	eng.LoadContract(contract, "etag-1")

	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation: "testOp",
		Input:     map[string]any{"customer.status": "blocked"},
		DryRun:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "would_deny" {
		t.Fatalf("expected would_deny, got %s", resp.Outcome)
	}
	if !resp.DryRun {
		t.Fatal("expected DryRun=true in response")
	}
}

func TestEngine_Evaluate_contractETagMismatchReturnsSystemError(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	eng.LoadContract(makeMinimalContract(), "etag-current")

	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation:    "testOp",
		ContractETag: "etag-stale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "system_error" {
		t.Fatalf("expected system_error, got %s", resp.Outcome)
	}
	if resp.Error == nil || resp.Error.Code != "CONTRACT_VERSION_MISMATCH" {
		t.Fatalf("expected CONTRACT_VERSION_MISMATCH, got %+v", resp.Error)
	}
	if resp.Error.HttpStatus != 409 {
		t.Fatalf("expected HTTP 409, got %d", resp.Error.HttpStatus)
	}
}

func TestEngine_Evaluate_unknownOperationReturnsError(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	eng.LoadContract(makeMinimalContract(), "etag-1")

	_, err := eng.Evaluate(context.Background(), &Request{Operation: "unknownOp"})
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}
}

func TestEngine_Evaluate_noContractReturnsError(t *testing.T) {
	eng := NewEngine(&mockPorts{})
	// No contract loaded.
	_, err := eng.Evaluate(context.Background(), &Request{Operation: "testOp"})
	if err == nil {
		t.Fatal("expected error when no contract is loaded")
	}
}

func TestEngine_Evaluate_portFactFetchedAndUsedInCondition(t *testing.T) {
	ports := &mockPorts{
		getFunc: func(_ context.Context, port, fact string, _ map[string]any) (any, error) {
			if port == "customerRepo" && fact == "customer.status" {
				return "active", nil
			}
			return nil, fmt.Errorf("unexpected port=%s fact=%s", port, fact)
		},
		executeFunc: func(_ context.Context, _, _ string, _ map[string]any) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}
	eng := NewEngine(ports)
	contract := &Contract{
		Facts: map[string]FactDef{
			"customer.status": {Source: "port:customerRepo", Required: true, OnMissing: "system_error"},
		},
		DerivedFacts: map[string]DerivedFactDef{},
		Rules: []RuleDef{
			{
				ID:   "deny-blocked",
				When: Condition{Fact: "customer.status", Equals: "blocked"},
				Verdict: VerdictDef{Deny: &DenyVerdict{
					Code:  "BLOCKED",
					Error: ErrorEnvelope{Code: "BLOCKED", HttpStatus: 403},
				}},
			},
		},
		Operations: map[string]OperationDef{
			"testOp": {ConstrainedBy: []string{"deny-blocked"}},
		},
		Entities: map[string]EntityDef{},
	}
	eng.LoadContract(contract, "etag-1")

	// "active" from port — deny rule should NOT fire.
	resp, err := eng.Evaluate(context.Background(), &Request{
		Operation: "testOp",
		Input:     map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome != "executed" {
		t.Fatalf("expected executed, got %s (error: %+v)", resp.Outcome, resp.Error)
	}
}
