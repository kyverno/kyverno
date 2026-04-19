package compiler

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies/v1alpha1"
)

func emptyPolicy() *policiesv1alpha1.AuthorizingPolicy {
	pol := &policiesv1alpha1.AuthorizingPolicy{}
	pol.Name = "test"
	return pol
}

func TestCompileEmptyPolicy(t *testing.T) {
	compiled, errs := NewCompiler().Compile(emptyPolicy())
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if compiled == nil {
		t.Fatal("expected compiled policy, got nil")
	}
	if len(compiled.Rules) != 0 {
		t.Fatalf("expected no rules, got %d", len(compiled.Rules))
	}
}

func TestCompileSimpleAllowRule(t *testing.T) {
	pol := emptyPolicy()
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "allow-all",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "true",
		},
	}

	compiled, errs := NewCompiler().Compile(pol)
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(compiled.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(compiled.Rules))
	}
	if compiled.Rules[0].Expression == nil {
		t.Fatal("expected non-nil expression program")
	}
	if compiled.Rules[0].Effect != policiesv1alpha1.AuthorizingRuleEffectAllow {
		t.Fatalf("expected Allow effect, got %s", compiled.Rules[0].Effect)
	}
}

func TestCompileVariables(t *testing.T) {
	pol := emptyPolicy()
	pol.Spec.Variables = []policiesv1alpha1.AuthorizingVariable{
		{Name: "myVar", Expression: "true"},
	}

	compiled, errs := NewCompiler().Compile(pol)
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if compiled.Variables == nil || compiled.Variables["myVar"] == nil {
		t.Fatal("expected variable 'myVar' to be compiled")
	}
}

func TestCompileInvalidExpression(t *testing.T) {
	pol := emptyPolicy()
	pol.Spec.Rules = []policiesv1alpha1.AuthorizingRule{
		{
			Name:       "bad-expr",
			Effect:     policiesv1alpha1.AuthorizingRuleEffectAllow,
			Expression: "!!!invalid!!!",
		},
	}

	_, errs := NewCompiler().Compile(pol)
	if errs == nil {
		t.Fatal("expected compile errors for invalid expression")
	}
}
