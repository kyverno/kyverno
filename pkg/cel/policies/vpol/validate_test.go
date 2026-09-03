package vpol

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		vpol    *v1beta1.ValidatingPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			vpol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "valid-policy",
				},
				Spec: v1beta1.ValidatingPolicySpec{
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
					Validations: []v1.Validation{
						{
							Expression: "object.spec.replicas > 0",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing matchConstraints",
			vpol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-constraints",
				},
				Spec: v1beta1.ValidatingPolicySpec{},
			},
			wantErr: true,
		},
		{
			name: "empty resourceRules",
			vpol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty-rules",
				},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: &v1.MatchResources{
						ResourceRules: []v1.NamedRuleWithOperations{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "only ExcludeResourceRules present (should fail)",
			vpol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exclude-only",
				},
				Spec: v1beta1.ValidatingPolicySpec{
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
			name: "invalid CEL validation expression",
			vpol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-cel",
				},
				Spec: v1beta1.ValidatingPolicySpec{
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
					Validations: []v1.Validation{
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
			warnings, err := Validate(tt.vpol)
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
