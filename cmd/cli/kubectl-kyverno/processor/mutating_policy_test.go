package processor

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Test_ApplyPoliciesOnResource_targetConstrainedMutatingPolicySkipsTrigger verifies that a
// MutatingPolicy with active targetMatchConstraints is not evaluated inline against its
// trigger, as admission does, so only the declared target receives the mutation.
func Test_ApplyPoliciesOnResource_targetConstrainedMutatingPolicySkipsTrigger(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "test", "cli", "test-mutating-policy", "mutate-existing")
	results, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	assert.NilError(t, err)
	assert.Equal(t, len(results.MutatingPolicies), 1)

	load := func(name string) *unstructured.Unstructured {
		data, err := os.ReadFile(filepath.Join(dir, name))
		assert.NilError(t, err)
		resources, err := resource.GetUnstructuredResources(data)
		assert.NilError(t, err)
		assert.Equal(t, len(resources), 1)
		return resources[0]
	}
	trigger := load("trigger.yaml")
	target := load("target.yaml")

	processor := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: results.MutatingPolicies,
		Resource:         *trigger,
		TargetResources:  []*unstructured.Unstructured{target},
		Variables:        &variables.Variables{},
		Rc:               &ResultCounts{},
		Out:              io.Discard,
	}

	responses, err := processor.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	// Only the mutate-existing evaluation of the target remains; an inline evaluation
	// would add a response for the trigger carrying the target-shaped patch.
	assert.Equal(t, len(responses), 1, "unexpected inline evaluation of trigger %s", trigger.GetName())
	assert.Equal(t, responses[0].Resource.GetKind(), target.GetKind())
	assert.Equal(t, responses[0].Resource.GetName(), target.GetName())
}

func Test_ApplyPoliciesOnResource_MAPParameterResolutionErrorPreserved(t *testing.T) {
	failPolicy := admissionregistrationv1beta1.Fail
	denyAction := admissionregistrationv1beta1.DenyAction
	mapPolicy := admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-map-param-error",
		},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			FailurePolicy: &failPolicy,
			ParamKind: &admissionregistrationv1beta1.ParamKind{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			MatchConstraints: &admissionregistrationv1beta1.MatchResources{
				ResourceRules: []admissionregistrationv1beta1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"configmaps"},
							},
						},
					},
				},
			},
			Mutations: []admissionregistrationv1beta1.Mutation{
				{
					PatchType: admissionregistrationv1beta1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1beta1.ApplyConfiguration{
						Expression: "Object{metadata: Object.metadata{labels: {'mutated': 'true'}}}",
					},
				},
			},
		},
	}
	binding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-map-binding",
		},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: "test-map-param-error",
			ParamRef: &admissionregistrationv1beta1.ParamRef{
				Name:                    "missing-cm",
				Namespace:               "default",
				ParameterNotFoundAction: &denyAction,
			},
		},
	}

	target := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "target-cm",
				"namespace": "default",
			},
		},
	}

	rc := &ResultCounts{}
	processor := PolicyProcessor{
		Store:                           &store.Store{},
		MutatingAdmissionPolicies:       []admissionregistrationv1beta1.MutatingAdmissionPolicy{mapPolicy},
		MutatingAdmissionPolicyBindings: []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{binding},
		Resource:                        *target,
		Variables:                       &variables.Variables{},
		Rc:                              rc,
		Out:                             io.Discard,
	}

	responses, err := processor.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Equal(t, len(responses), 1)
	assert.Equal(t, len(responses[0].PolicyResponse.Rules), 1)
	assert.Equal(t, responses[0].PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
	assert.Equal(t, rc.Error, 1)
}
