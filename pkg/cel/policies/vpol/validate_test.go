package vpol

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Validate_NamespacedPolicyRejectsGlobalContext(t *testing.T) {
	vpol := &v1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gctx", Namespace: "tenant-ns"},
		Spec: v1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, Resources: []string{"pods"}},
					},
				}},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: `globalContext.get("cluster-entry", "") == null`},
			},
		},
	}

	_, err := Validate(vpol)
	assert.ErrorContains(t, err, "globalContext.* is not allowed in namespaced policies")
}

func Test_Validate_NamespacedPolicyHTTPToggle(t *testing.T) {
	t.Cleanup(func() { _ = toggle.AllowHTTPInNamespacedPolicies.Parse("") })

	for _, tt := range []struct {
		toggle  string
		wantErr bool
	}{{toggle: "false", wantErr: true}, {toggle: "true", wantErr: false}} {
		for _, expr := range []string{
			`http.Get('https://example.com') != null`,
			`object.?data.?x.orValue(http.Get('https://example.com')) != null`,
		} {
			t.Run("allowHTTP="+tt.toggle+"/"+expr, func(t *testing.T) {
				require.NoError(t, toggle.AllowHTTPInNamespacedPolicies.Parse(tt.toggle))
				vpol := &v1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "probe", Namespace: "tenant-ns"},
					Spec: v1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, Resources: []string{"configmaps"}},
								},
							}},
						},
						Validations: []admissionregistrationv1.Validation{{Expression: expr}},
					},
				}

				_, err := Validate(vpol)
				if tt.wantErr {
					assert.ErrorContains(t, err, "http.* is not allowed in namespaced policies")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}
}
