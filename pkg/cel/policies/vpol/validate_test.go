package vpol

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
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
