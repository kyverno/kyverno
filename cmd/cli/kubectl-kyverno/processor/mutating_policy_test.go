package processor

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	ignorePolicy := admissionregistrationv1beta1.Ignore
	denyAction := admissionregistrationv1beta1.DenyAction
	allowAction := admissionregistrationv1beta1.AllowAction

	tests := []struct {
		name          string
		failurePolicy *admissionregistrationv1beta1.FailurePolicyType
		notFound      *admissionregistrationv1beta1.ParameterNotFoundActionType
		expectError   bool
	}{
		{
			name:          "failurePolicy Fail with DenyAction records error and avoids success output",
			failurePolicy: &failPolicy,
			notFound:      &denyAction,
			expectError:   true,
		},
		{
			name:          "failurePolicy Fail with AllowAction skips binding with no error",
			failurePolicy: &failPolicy,
			notFound:      &allowAction,
			expectError:   false,
		},
		{
			name:          "failurePolicy Ignore with DenyAction suppresses error",
			failurePolicy: &ignorePolicy,
			notFound:      &denyAction,
			expectError:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mapPolicy := admissionregistrationv1beta1.MutatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-map-param-error",
				},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
					FailurePolicy: tc.failurePolicy,
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
						ParameterNotFoundAction: tc.notFound,
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

			var outBuf bytes.Buffer
			rc := &ResultCounts{}
			processor := PolicyProcessor{
				Store:                           &store.Store{},
				MutatingAdmissionPolicies:       []admissionregistrationv1beta1.MutatingAdmissionPolicy{mapPolicy},
				MutatingAdmissionPolicyBindings: []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{binding},
				Resource:                        *target,
				Variables:                       &variables.Variables{},
				Rc:                              rc,
				Out:                             &outBuf,
			}

			responses, err := processor.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Assert(t, !strings.Contains(outBuf.String(), "Mutation has been applied successfully"))

			if tc.expectError {
				assert.Equal(t, len(responses), 1)
				assert.Equal(t, len(responses[0].PolicyResponse.Rules), 1)
				assert.Equal(t, responses[0].PolicyResponse.Rules[0].Status(), engineapi.RuleStatusError)
				assert.Equal(t, rc.Error, 1)
			} else {
				assert.Equal(t, rc.Error, 0)
				if len(responses) > 0 {
					for _, r := range responses[0].PolicyResponse.Rules {
						assert.Assert(t, r.Status() != engineapi.RuleStatusError)
					}
				}
			}
		})
	}
}

func Test_ApplyPoliciesOnResource_NativePolicies_OperationFiltering(t *testing.T) {
	mapPolicy := admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-update-only-map",
		},
		Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1beta1.MatchResources{
				ResourceRules: []admissionregistrationv1beta1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Update,
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
						Expression: "Object{metadata: Object.metadata{labels: {'updated': 'true'}}}",
					},
				},
			},
		},
	}
	vapPolicy := admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-update-only-vap",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Update,
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
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.metadata.name == oldObject.metadata.name",
				},
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

	t.Run("operation CREATE skips UPDATE-only native policies", func(t *testing.T) {
		rc := &ResultCounts{}
		processor := PolicyProcessor{
			Store:                       &store.Store{},
			Operation:                   "CREATE",
			MutatingAdmissionPolicies:   []admissionregistrationv1beta1.MutatingAdmissionPolicy{mapPolicy},
			ValidatingAdmissionPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{vapPolicy},
			Resource:                    *target,
			Variables:                   &variables.Variables{},
			Rc:                          rc,
			Out:                         io.Discard,
		}
		responses, err := processor.ApplyPoliciesOnResource()
		assert.NilError(t, err)
		assert.Equal(t, rc.Pass, 0)
		assert.Equal(t, rc.Error, 0)
		assert.Equal(t, rc.Fail, 0)
		for _, r := range responses {
			assert.Assert(t, r.IsEmpty())
		}
	})

	t.Run("operation UPDATE evaluates UPDATE-only native policies", func(t *testing.T) {
		rc := &ResultCounts{}
		processor := PolicyProcessor{
			Store:                       &store.Store{},
			Operation:                   "UPDATE",
			MutatingAdmissionPolicies:   []admissionregistrationv1beta1.MutatingAdmissionPolicy{mapPolicy},
			ValidatingAdmissionPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{vapPolicy},
			Resource:                    *target,
			Variables:                   &variables.Variables{},
			Rc:                          rc,
			Out:                         io.Discard,
		}
		responses, err := processor.ApplyPoliciesOnResource()
		assert.NilError(t, err)
		assert.Equal(t, len(responses), 2)
		assert.Equal(t, responses[0].PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)
		assert.Equal(t, responses[1].PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)
		assert.Equal(t, responses[0].PatchedResource.GetLabels()["updated"], "true")
	})
}

func Test_ApplyPoliciesOnResource_NamespaceDelete_SelectorMatching(t *testing.T) {
	vapPolicy := admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deny-prod-namespace-delete",
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Delete,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"namespaces"},
							},
						},
					},
				},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"env": "prod",
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "false",
					Message:    "cannot delete prod namespace",
				},
			},
		},
	}

	testCases := []struct {
		name       string
		labels     map[string]interface{}
		expectFail bool
	}{
		{
			name: "env=prod matches selector and fails admission on DELETE",
			labels: map[string]interface{}{
				"env": "prod",
			},
			expectFail: true,
		},
		{
			name: "env=dev does not match selector and skips policy on DELETE",
			labels: map[string]interface{}{
				"env": "dev",
			},
			expectFail: false,
		},
		{
			name:       "absent labels does not match selector and skips policy on DELETE",
			labels:     nil,
			expectFail: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := map[string]interface{}{
				"name": "test-ns",
			}
			if tc.labels != nil {
				metadata["labels"] = tc.labels
			}
			nsObj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata":   metadata,
				},
			}

			rc := &ResultCounts{}
			processor := PolicyProcessor{
				Store:                       &store.Store{},
				Operation:                   "DELETE",
				ValidatingAdmissionPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{vapPolicy},
				Resource:                    *nsObj,
				Variables:                   &variables.Variables{},
				Rc:                          rc,
				Out:                         io.Discard,
			}

			responses, err := processor.ApplyPoliciesOnResource()
			assert.NilError(t, err)

			if tc.expectFail {
				assert.Equal(t, len(responses), 1)
				assert.Equal(t, len(responses[0].PolicyResponse.Rules), 1)
				assert.Equal(t, responses[0].PolicyResponse.Rules[0].Status(), engineapi.RuleStatusFail)
				assert.Equal(t, rc.Fail, 1)
			} else {
				assert.Equal(t, rc.Fail, 0)
				for _, r := range responses {
					assert.Assert(t, r.IsEmpty())
				}
			}
		})
	}
}
