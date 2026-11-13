package compiler

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// the exception env deliberately omits `variables` and `exceptions`; both must stay declared for
// the policy's own expressions, which share the builder that narrows them away.
func TestCompile_PolicyScopedIdentifiersStayDeclared(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{{
		name:       "exceptions",
		expression: "string(object.spec.image) in exceptions.allowedImages",
	}, {
		name:       "exceptions allowedValues",
		expression: "'CHOWN' in exceptions.allowedValues",
	}, {
		name:       "variables",
		expression: "'CHOWN' in variables.allowedCapabilities",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					Variables: []admissionregistrationv1.Variable{
						{Name: "allowedCapabilities", Expression: "['CHOWN']"},
					},
					Validations: []admissionregistrationv1.Validation{{Expression: tt.expression}},
				},
			}
			_, errs := NewCompiler().Compile(policy, nil)
			assert.Assert(t, errs == nil, "expected %q to compile, got %v", tt.expression, errs)
		})
	}
}

func TestCompile_ForeignEvaluationMode(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "envoy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Envoy"},
			Validations:             []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	compiled, errs := NewCompiler().Compile(policy, nil)
	assert.Assert(t, compiled == nil)
	assert.Assert(t, len(errs) > 0)
}

func TestCompile_SelectsKyvernoExceptions(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "owned"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	invalid := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "owned-exception"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "bad",
				Expression: "???",
			}},
		},
	}
	foreign := invalid.DeepCopy()
	foreign.Name = "envoy-exception"
	foreign.Spec.EvaluationMode = "Envoy"

	compiled, errs := NewCompiler().Compile(policy, []*policiesv1beta1.PolicyException{foreign})
	assert.Assert(t, compiled != nil)
	assert.Assert(t, errs == nil)

	compiled, errs = NewCompiler().Compile(policy, []*policiesv1beta1.PolicyException{foreign, invalid})
	assert.Assert(t, compiled == nil)
	assert.Assert(t, len(errs) > 0)
}
