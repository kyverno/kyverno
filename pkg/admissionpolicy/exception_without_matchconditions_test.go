package admissionpolicy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A PolicyException with no matchConditions and no images or allowedValues is a
// full exemption in the CEL engine. Native policies express it as an
// always-false match condition, because their match conditions are ANDed.
func TestBuildNativePolicy_ExceptionExclusionConditions(t *testing.T) {
	tests := []struct {
		name            string
		matchConditions []admissionregistrationv1.MatchCondition
		images          []string
		allowedValues   []string
		wantExpressions []string
	}{
		{
			name:            "blanket exception exempts every request",
			wantExpressions: []string{"false"},
		},
		{
			name:            "exception with images is not a blanket exemption",
			images:          []string{"ghcr.io/example/app:1.0.0"},
			wantExpressions: nil,
		},
		{
			name:            "exception with allowed values is not a blanket exemption",
			allowedValues:   []string{"team-a"},
			wantExpressions: nil,
		},
		{
			name:            "exception with match conditions keeps the negated condition",
			matchConditions: []admissionregistrationv1.MatchCondition{{Name: "skip-app", Expression: "object.metadata.name == 'skip'"}},
			wantExpressions: []string{"!(object.metadata.name == 'skip')"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exception := &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Name: "exempt-everything", Namespace: "default"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: "deny-all-pods", Kind: "ValidatingPolicy"}},
					MatchConditions: tt.matchConditions,
					Images:          tt.images,
					AllowedValues:   tt.allowedValues,
				},
			}

			t.Run("ValidatingAdmissionPolicy", func(t *testing.T) {
				vpol := &policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "deny-all-pods", UID: "vpol-uid"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "false", Message: "pods are denied"},
						},
					},
				}
				vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
				err := BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol), []engineapi.GenericException{
					engineapi.NewCELPolicyException(exception),
				})
				require.NoError(t, err)
				require.Len(t, vap.Spec.MatchConditions, len(tt.wantExpressions))
				for i, expression := range tt.wantExpressions {
					assert.Equal(t, expression, vap.Spec.MatchConditions[i].Expression)
					assert.NotEmpty(t, vap.Spec.MatchConditions[i].Name)
				}
			})

			t.Run("MutatingAdmissionPolicy", func(t *testing.T) {
				mpol := &policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "deny-all-pods", UID: "mpol-uid"},
					Spec: policiesv1beta1.MutatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
										},
									},
								},
							},
						},
					},
				}
				mapol := &admissionregistrationv1alpha1.MutatingAdmissionPolicy{}
				BuildMutatingAdmissionPolicy(mapol, mpol, []policiesv1beta1.PolicyException{*exception})
				require.Len(t, mapol.Spec.MatchConditions, len(tt.wantExpressions))
				for i, expression := range tt.wantExpressions {
					assert.Equal(t, expression, mapol.Spec.MatchConditions[i].Expression)
					assert.NotEmpty(t, mapol.Spec.MatchConditions[i].Name)
				}
			})
		})
	}
}
