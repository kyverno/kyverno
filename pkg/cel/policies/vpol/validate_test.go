package vpol

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func validMatchConstraints() *v1.MatchResources {
	return &v1.MatchResources{
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
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		pol     v1beta1.ValidatingPolicyLike
		wantErr bool
	}{
		{
			name: "valid policy",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-vpol"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
				},
			},
			wantErr: false,
		},
		{
			name: "missing matchConstraints",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "no-constraints"},
				Spec:       v1beta1.ValidatingPolicySpec{},
			},
			wantErr: true,
		},
		{
			name: "empty resourceRules",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-rules"},
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
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "exclude-only"},
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
			name: "invalid policy",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-policy"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
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
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-vpol-with-cel"},
				Spec: v1beta1.ValidatingPolicySpec{
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
		{
			name: "invalid validation expression",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-validation"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
					Validations: []v1.Validation{
						{
							Expression: "object.metadata.name ==",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid messageExpression",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-message-expression"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
					Validations: []v1.Validation{
						{
							Expression:        `object.metadata.name != ''`,
							MessageExpression: "not valid cel",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid audit annotation",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "invalid-audit"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
					AuditAnnotations: []v1.AuditAnnotation{
						{
							Key:             "owner",
							ValueExpression: "object.metadata.labels[",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid policy with validations and audit annotations",
			pol: &v1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "valid-full"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
					Validations: []v1.Validation{
						{
							Expression: `object.metadata.name != ''`,
							Message:    "name required",
						},
					},
					AuditAnnotations: []v1.AuditAnnotation{
						{
							Key:             "owner",
							ValueExpression: `"team-a"`,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "namespaced policy with http in validation",
			pol: &v1beta1.NamespacedValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "http-policy", Namespace: "test"},
				Spec: v1beta1.ValidatingPolicySpec{
					MatchConstraints: validMatchConstraints(),
					Validations: []v1.Validation{
						{
							Expression: `http.Get('https://example.com') != null`,
							Message:    "http not allowed",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "namespaced policy with http in validation" {
				require.NoError(t, toggle.AllowHTTPInNamespacedPolicies.Parse("false"))
				t.Cleanup(func() { _ = toggle.AllowHTTPInNamespacedPolicies.Parse("false") })
			}

			warnings, err := Validate(tt.pol.DeepCopyObject().(v1beta1.ValidatingPolicyLike))

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
