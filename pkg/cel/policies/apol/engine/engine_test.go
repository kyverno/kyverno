package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
)

func compilePolicy(t *testing.T, pol *policiesv1alpha1.AuthorizingPolicy) *apolcompiler.Policy {
	t.Helper()
	compiled, errs := apolcompiler.NewCompiler().Compile(pol)
	if errs != nil {
		t.Fatalf("compile errors: %v", errs)
	}
	return compiled
}

func requestActivation() map[string]any {
	return map[string]any{
		"user":   "alice",
		"verb":   "get",
		"groups": []any{"system:masters"},
	}
}

func TestEngineNoOpinionOnEmptyPolicy(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "empty"
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	decision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectNoOpinion {
		t.Fatalf("expected NoOpinion, got %s", decision.Effect)
	}
}

func TestEngineAllowRule(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "allow-all"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "always-allow",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "true",
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	decision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectAllow {
		t.Fatalf("expected Allow, got %s", decision.Effect)
	}
}

func TestEngineAllowRuleUsingVariables(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "allow-with-vars"
	pol.Spec.Variables = []policiesv1alpha1.AuthorizingVariable{
		{
			Name:       "isAlice",
			Expression: "request.user == 'alice'",
		},
		{
			Name:       "isReadVerb",
			Expression: "request.verb == 'get'",
		},
	}
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "allow-alice-read",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "variables.isAlice && variables.isReadVerb",
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	decision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectAllow {
		t.Fatalf("expected Allow, got %s (reason: %s)", decision.Effect, decision.Reason)
	}
}

func TestEngineDenyRule(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "deny-all"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "always-deny",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectDeny,
			Expression: "true",
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	decision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectDeny {
		t.Fatalf("expected Deny, got %s", decision.Effect)
	}
}

func TestEngineRuleWithFalseExpressionSkipped(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "skip-false"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "skip-me",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "false",
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	decision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectNoOpinion {
		t.Fatalf("expected NoOpinion, got %s", decision.Effect)
	}
}

func TestEngineRuleExpressionErrorCarriesEvaluationError(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "erroring-rule"
	pol.Spec.Variables = []policiesv1alpha1.AuthorizingVariable{
		{
			Name:       "bad",
			Expression: "1 / 0",
		},
	}
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "runtime-error",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "variables.bad == 1",
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	decision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectNoOpinion {
		t.Fatalf("expected NoOpinion, got %s", decision.Effect)
	}
	if decision.EvaluationError == "" {
		t.Fatalf("expected evaluationError to be set")
	}
}

func TestEngineConditionalRule(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "conditional"
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "with-conds",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectConditional,
			Expression: "true",
			Conditions: []policiesv1alpha1.AuthorizingCondition{
				{
					ID:         "cond-allow",
					Expression: "true",
					Effect:     policiesv1alpha1.AuthorizingConditionEffectAllow,
				},
				{
					ID:         "cond-false",
					Expression: "false",
					Effect:     policiesv1alpha1.AuthorizingConditionEffectDeny,
				},
			},
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)
	sarDecision, err := eng.HandleSAR(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error from HandleSAR: %v", err)
	}
	if sarDecision.Effect != policiesv1alpha1.AuthorizingRuleEffectConditional {
		t.Fatalf("expected Conditional from HandleSAR, got %s", sarDecision.Effect)
	}
	if len(sarDecision.ConditionSet) != 2 {
		t.Fatalf("expected 2 conditions in condition set, got %d", len(sarDecision.ConditionSet))
	}
	if sarDecision.ConditionSet[0].ID != "cond-allow" {
		t.Fatalf("expected first condition id 'cond-allow', got %s", sarDecision.ConditionSet[0].ID)
	}
	if sarDecision.ConditionSet[0].Condition == "" {
		t.Fatalf("expected serialized condition expression in condition set")
	}

	decision, err := eng.HandleConditionsReview(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectAllow {
		t.Fatalf("expected Allow, got %s", decision.Effect)
	}
	if len(decision.ConditionResults) != 1 {
		t.Fatalf("expected 1 truthy condition result, got %d", len(decision.ConditionResults))
	}
	if decision.ConditionResults[0].ID != "cond-allow" {
		t.Fatalf("expected condition id 'cond-allow', got %s", decision.ConditionResults[0].ID)
	}
}

func TestEngineConditionalRule_AllowErrorIgnored(t *testing.T) {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "conditional-error-allow"
	pol.Spec.FailurePolicy = policiesv1alpha1.AuthorizingFailurePolicyDeny
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "with-conds",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectConditional,
			Expression: "true",
			Conditions: []policiesv1alpha1.AuthorizingCondition{
				{
					ID:         "broken-allow",
					Expression: "request.missing.field == 'x'",
					Effect:     policiesv1alpha1.AuthorizingConditionEffectAllow,
				},
			},
		},
	}
	compiled := compilePolicy(t, pol)
	eng := NewEngine(compiled)

	decision, err := eng.HandleConditionsReview(context.Background(), requestActivation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Effect != policiesv1alpha1.AuthorizingRuleEffectNoOpinion {
		t.Fatalf("expected NoOpinion for allow-condition error, got %s", decision.Effect)
	}
}
