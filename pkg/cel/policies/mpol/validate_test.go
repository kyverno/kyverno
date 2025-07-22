package mpol

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		pol     *v1alpha1.MutatingPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			pol: &v1alpha1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-mpol",
				},
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ResourceRules: []v1.NamedRuleWithOperations{
							{
								RuleWithOperations: v1.RuleWithOperations{
									Rule: v1.Rule{
										APIGroups: []string{"apps"},
										Resources: []string{"deployments"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing matchConstraints",
			pol: &v1alpha1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-constraints",
				},
				Spec: v1alpha1.MutatingPolicySpec{},
			},
			wantErr: true,
		},
		{
			name: "empty resourceRules",
			pol: &v1alpha1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-rules",
				},
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ResourceRules: []v1.NamedRuleWithOperations{ /* empty config */ },
					},
				},
			},
			wantErr: true,
		},
		{
			name: "only ExcludeResourceRules present (should fail)",
			pol: &v1alpha1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exclude-only",
				},
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ExcludeResourceRules: []v1.NamedRuleWithOperations{
							{
								RuleWithOperations: v1.RuleWithOperations{
									Rule: v1.Rule{
										APIGroups: []string{"batch"},
										Resources: []string{"jobs"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid policy",
			pol: &v1alpha1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-policy",
				},
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ResourceRules: []v1.NamedRuleWithOperations{
							{
								RuleWithOperations: v1.RuleWithOperations{
									Rule: v1.Rule{
										APIGroups: []string{"apps"},
										Resources: []string{"deployments"},
									},
								},
							},
						},
					},
					MatchConditions: []v1.MatchCondition{
						{
							Name:       "invalid-cel",
							Expression: "this is not a CEL expression",
						},
					},
					FailurePolicy: func() *v1.FailurePolicyType {
						fp := v1.Fail
						return &fp
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "valid policy with match conditions",
			pol: &v1alpha1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-mpol-with-cel",
				},
				Spec: v1alpha1.MutatingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ResourceRules: []v1.NamedRuleWithOperations{
							{
								RuleWithOperations: v1.RuleWithOperations{
									Rule: v1.Rule{
										APIGroups: []string{""},
										Resources: []string{"pods"},
									},
								},
							},
						},
					},
					MatchConditions: []v1.MatchCondition{
						{
							Name:       "isProd",
							Expression: `object.metadata.labels["env"] == "prod"`,
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := Validate(tt.pol)

			if tt.wantErr {
				assert.Error(t, err, "expected error but got nil for test: %s", tt.name)
				assert.NotEmpty(t, warnings, "expected warnings but got none")
			} else {
				assert.NoError(t, err, "expected no error but got %v", err)
				assert.Empty(t, warnings, "expected no warnings but got %#v", warnings)
			}
		})
	}
}
