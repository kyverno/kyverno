package compiler

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admissionregistration/v1"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name    string
		pol     *policiesv1beta1.MutatingPolicy
		polex   []*policiesv1alpha1.PolicyException
		wantErr bool
	}{
		{
			name: "valid applyConfiguration mutation",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					Mutations: []policiesv1beta1.Mutation{
						{
							Expression: `Object{spec: Object.spec{containers: [Object.spec.containers{name: "nginx"}]}}`,
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "invalid-expression in exception",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{},
			},
			polex: []*policiesv1alpha1.PolicyException{
				{
					Spec: policiesv1alpha1.PolicyExceptionSpec{
						MatchConditions: []admissionv1.MatchCondition{
							{Expression: "invalid && expression"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid mutation",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					Mutations: []policiesv1beta1.Mutation{
						{
							Expression: `Object{spec: Object.spec{restartPolicy: "Always"}}`,
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "policy with variables",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					Variables: []admissionv1.Variable{
						{
							Name:       "foo",
							Expression: "object.metadata.name",
						},
					},
					Mutations: []policiesv1beta1.Mutation{
						{
							Expression: `Object{}`,
						},
					},
				},
			},
			polex:   nil,
			wantErr: false,
		},
		{
			name: "valid matchCondition expression",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					MatchConditions: []admissionv1.MatchCondition{
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
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					MatchConditions: []admissionv1.MatchCondition{
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
		{
			name: "invalid mutation expression with unsupported operator",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					Mutations: []policiesv1beta1.Mutation{
						{
							Expression: `contains(object.metadata.labels) ?? Object{metadata: Object.metadata{labels: {"managed": "true"}}} : Object{}`,
						},
					},
				},
			},
			polex:   nil,
			wantErr: true,
		},
		{
			name: "invalid mutation expression",
			pol: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					Mutations: []policiesv1beta1.Mutation{
						{
							Expression: `invalid{spec: Object.spec{containers: [Object.spec.containers{name: "nginx"}]}}`,
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
