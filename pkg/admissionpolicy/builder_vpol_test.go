package admissionpolicy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The policy comes from the informer cache, so building a VAP must not rewrite it.
func TestBuildValidatingAdmissionPolicyKeepsSourcePolicy(t *testing.T) {
	t.Parallel()
	expression := "exceptions.allowedImages.size() > 0"
	nvpol := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "team-a"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			// spare capacity would let appends write into the cached backing arrays
			MatchConditions: make([]admissionregistrationv1.MatchCondition, 0, 4),
			Variables:       make([]admissionregistrationv1.Variable, 0, 4),
			Validations:     []admissionregistrationv1.Validation{{Expression: expression}},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "skip", Expression: "true"}},
			Images:          []string{"ghcr.io/example/app"},
		},
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}

	err := BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewNamespacedValidatingPolicy(nvpol), []engineapi.GenericException{engineapi.NewCELPolicyException(exception)})
	require.NoError(t, err)

	assert.Equal(t, "variables.allowedImages.size() > 0", vap.Spec.Validations[0].Expression)
	assert.Len(t, vap.Spec.MatchConditions, 1)
	assert.Len(t, vap.Spec.Variables, 1)
	assert.Equal(t, expression, nvpol.Spec.Validations[0].Expression, "the source policy must not be rewritten")
	assert.Empty(t, nvpol.Spec.MatchConditions[:cap(nvpol.Spec.MatchConditions)][0].Name)
	assert.Empty(t, nvpol.Spec.Variables[:cap(nvpol.Spec.Variables)][0].Name)
}
