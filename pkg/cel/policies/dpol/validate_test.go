package dpol

import (
	"testing"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		dpol    *v1alpha1.DeletingPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			dpol: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-policy",
				},
				Spec: v1alpha1.DeletingPolicySpec{
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
			dpol: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-constraints",
				},
				Spec: v1alpha1.DeletingPolicySpec{},
			},
			wantErr: true,
		},
		{
			name: "empty resourceRules",
			dpol: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-rules",
				},
				Spec: v1alpha1.DeletingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ResourceRules: []v1.NamedRuleWithOperations{ /*empty config*/ },
					},
				},
			},
			wantErr: true,
		},
		{
			name: "only ExcludeResourceRules present (should fail)",
			dpol: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exclude-only",
				},
				Spec: v1alpha1.DeletingPolicySpec{
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
			dpol: &v1alpha1.DeletingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-cel",
				},
				Spec: v1alpha1.DeletingPolicySpec{
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
					Conditions: []v1.MatchCondition{
						{
							Expression: "this is not CEL",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := Validate(tt.dpol)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if len(warnings) == 0 {
					t.Errorf("expected warnings but got none")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				if len(warnings) != 0 {
					t.Errorf("expected no warnings but got: %v", warnings)
				}
			}
		})
	}
}
