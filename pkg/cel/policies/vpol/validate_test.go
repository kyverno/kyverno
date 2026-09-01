package vpol

import (
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Validate_NamespacedPolicyRejectsGlobalContext(t *testing.T) {
	vpol := &v1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-globalcontext-vpol",
			Namespace: "tenant-ns",
		},
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
			Variables: []v1.Variable{{
				Name:       "gctx",
				Expression: `globalContext.get("cluster-entry", "")`,
			}},
		},
	}

	warnings, err := Validate(vpol)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "globalContext.* is not allowed in namespaced policies")
	assert.NotEmpty(t, warnings)
}
