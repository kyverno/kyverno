package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name    string
		pol     *v1alpha1.MutatingPolicy
		polex   []*v1alpha1.PolicyException
		wantErr bool
	}{
		{
			name: "valid applyConfiguration mutation",
			pol: &v1alpha1.MutatingPolicy{
				Spec: v1alpha1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{
						{
							PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
							ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
								Expression: `Object{spec: Object.spec{containers: [Object.spec.containers{name: "nginx"}]}}`,
							},
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "invalid-expression in exception",
			pol: &v1alpha1.MutatingPolicy{
				Spec: v1alpha1.MutatingPolicySpec{},
			},
			polex: []*v1alpha1.PolicyException{
				{
					Spec: v1alpha1.PolicyExceptionSpec{
						MatchConditions: []admissionv1.MatchCondition{
							{Expression: "invalid && expression"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid jsonpatch mutation",
			pol: &v1alpha1.MutatingPolicy{
				Spec: v1alpha1.MutatingPolicySpec{
					Mutations: []admissionregistrationv1alpha1.Mutation{
						{
							PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
							JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
								Expression: `[{"op": "add", "path": "/spec/restartPolicy", "value": "Always"}]`,
							},
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "policy with variables",
			pol: &v1alpha1.MutatingPolicy{
				Spec: v1alpha1.MutatingPolicySpec{
					Variables: []admissionregistrationv1alpha1.Variable{
						{
							Name:       "foo",
							Expression: "object.metadata.name",
						},
					},
					Mutations: []admissionregistrationv1alpha1.Mutation{
						{
							PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
							ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
								Expression: `Object{}`,
							},
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "valid matchCondition expression",
			pol: &v1alpha1.MutatingPolicy{
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConditions: []admissionregistrationv1alpha1.MatchCondition{
						{
							Name:       "ns-is-test",
							Expression: `true`,
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "invalid matchCondition expression",
			pol: &v1alpha1.MutatingPolicy{
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConditions: []admissionregistrationv1alpha1.MatchCondition{
						{
							Name:       "ns-is-test",
							Expression: `this is not a cel`,
						},
					},
				},
			},
			polex:   nil,
			wantErr: true,
		},
	}

	compiler := NewCompiler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, errs := compiler.Compile(tt.pol, tt.polex)
			if tt.wantErr {
				assert.NotEmpty(t, errs, "expected error but got none")
			} else {
				assert.Empty(t, errs, "expected no error but got some: %v", errs)
				assert.NotNil(t, compiled, "expected compiled policy but got nil")
			}
		})
	}
}
