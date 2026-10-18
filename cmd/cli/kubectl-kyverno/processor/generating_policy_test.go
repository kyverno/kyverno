package processor

import (
	"io"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Test_ApplyPoliciesOnResource_preservesSkippedGeneratingPolicy verifies that a skipped
// GeneratingPolicy still produces an engine response for CLI test result handling.
func Test_ApplyPoliciesOnResource_preservesSkippedGeneratingPolicy(t *testing.T) {
	policy := &policiesv1beta1.GeneratingPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policies.kyverno.io/v1beta1",
			Kind:       "GeneratingPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "generate-configmap-for-workloads",
		},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"namespaces"},
							},
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "workload-namespaces-only",
					Expression: `object.metadata.?labels["example.com/type"].orValue("") == "workload"`,
				},
			},
		},
	}

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": "excluded-namespace",
				"labels": map[string]interface{}{
					"example.com/type": "platform",
				},
			},
		},
	}

	processor := PolicyProcessor{
		Store:              &store.Store{},
		GeneratingPolicies: []policiesv1beta1.GeneratingPolicyLike{policy},
		Resource:           resource,
		Variables:          &variables.Variables{},
		Rc:                 &ResultCounts{},
		Out:                io.Discard,
	}

	responses, err := processor.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Equal(t, len(responses), 1)
	assert.Equal(t, responses[0].Policy().GetName(), policy.Name)
	assert.Equal(t, len(responses[0].PolicyResponse.Rules), 0)
}
