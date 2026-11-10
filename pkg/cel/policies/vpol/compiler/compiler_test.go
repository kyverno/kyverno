package compiler

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

func TestCompile_PostIsReadOnlyOutsideValidations(t *testing.T) {
	t.Parallel()
	post := `has(resource.post("v1", "configmaps", {}).metadata)`
	tests := []struct {
		name      string
		spec      policiesv1beta1.ValidatingPolicySpec
		wantField string
	}{{
		name: "variable",
		spec: policiesv1beta1.ValidatingPolicySpec{
			Variables:   []admissionregistrationv1.Variable{{Name: "created", Expression: post}},
			Validations: []admissionregistrationv1.Validation{{Expression: "true"}},
		},
		wantField: "spec.variables[0].expression",
	}, {
		name: "match condition",
		spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "created", Expression: post}},
			Validations:     []admissionregistrationv1.Validation{{Expression: "true"}},
		},
		wantField: "spec.matchConditions[0].expression",
	}, {
		name: "validation",
		spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: post}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			policy := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       tt.spec,
			}
			_, errs := NewCompiler().Compile(policy, nil)
			if tt.wantField == "" {
				assert.Assert(t, errs == nil, "expected post to compile, got %v", errs)
				return
			}
			found := false
			for _, err := range errs {
				if err.Type == field.ErrorTypeForbidden && err.Field == tt.wantField {
					found = true
				}
			}
			assert.Assert(t, found, "expected a forbidden error on %s, got %v", tt.wantField, errs)
		})
	}
}
