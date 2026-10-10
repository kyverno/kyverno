package autogen

import (
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pod controller autogen is a Kubernetes admission concept; a JSON mode policy
// never produces generated rules even if its spec would otherwise qualify.
func TestAutogen_JSONModeProducesNothing(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "p"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
					},
				}},
			},
			AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{Controllers: []string{"deployments"}},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: `[JSONPatch{op:"add",path:"/metadata/labels",value:{"a":"b"}}]`},
			}},
		},
	}
	generated, err := Autogen(policy)
	require.NoError(t, err)
	require.Len(t, generated, 1, "Kubernetes mode policy autogens for deployments")

	policy.Spec.EvaluationConfiguration = &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON}
	generated, err = Autogen(policy)
	require.NoError(t, err)
	require.Empty(t, generated)
}
