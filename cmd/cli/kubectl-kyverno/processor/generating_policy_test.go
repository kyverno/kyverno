package processor

import (
	"io"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Test_ApplyPoliciesOnResource_preservesSkippedGeneratingPolicy verifies that a skipped
// GeneratingPolicy produces a real Skip rule response (gpol's engine now sets one, matching
// vpol/mpol - see pkg/cel/policies/gpol/engine/engine.go), not just an engine response with no
// rules at all. A response with zero rules was enough for the CLI test command's result matcher
// (what this test originally pinned down, kyverno#17382/#17388), but it left `kyverno apply`'s
// pass/fail/warn/error/skip tally and both its plain-text and --table output printing nothing
// for this resource, with no way to tell "correctly skipped" apart from "something broke".
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
	assert.Equal(t, len(responses[0].PolicyResponse.Rules), 1)
	assert.Equal(t, responses[0].PolicyResponse.Rules[0].Status(), engineapi.RuleStatusSkip)
}

// Test_ApplyPoliciesOnResource_NamespacedGeneratingPolicy verifies that a NamespacedGeneratingPolicy
// is evaluated across success, skip, and error outcomes via the fallback compilation path
// without panicking due to typed-nil conversion defects.
func Test_ApplyPoliciesOnResource_NamespacedGeneratingPolicy(t *testing.T) {
	matchConstraints := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				},
			},
		},
	}

	tests := []struct {
		name                  string
		policy                *policiesv1beta1.NamespacedGeneratingPolicy
		resource              unstructured.Unstructured
		wantRulesCount        int
		wantRuleStatus        engineapi.RuleStatus
		wantIsSuccessful      bool
		wantGenResourcesCount int
		wantGenResourceName   string
		wantPassCount         int
		wantErrorCount        int
	}{
		{
			name: "successful generation",
			policy: &policiesv1beta1.NamespacedGeneratingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policies.kyverno.io/v1beta1",
					Kind:       "NamespacedGeneratingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "generate-cm-success",
					Namespace: "test-ns",
				},
				Spec: policiesv1beta1.GeneratingPolicySpec{
					MatchConstraints: matchConstraints,
					Generation: []policiesv1beta1.Generation{
						{
							Expression: `generator.Apply([{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "gen-child-cm"})}])`,
						},
					},
				},
			},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "trigger-cm",
						"namespace": "test-ns",
					},
				},
			},
			wantRulesCount:        1,
			wantRuleStatus:        engineapi.RuleStatusPass,
			wantIsSuccessful:      true,
			wantGenResourcesCount: 1,
			wantGenResourceName:   "gen-child-cm",
			wantPassCount:         1,
			wantErrorCount:        0,
		},
		{
			name: "false match condition skips policy",
			policy: &policiesv1beta1.NamespacedGeneratingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policies.kyverno.io/v1beta1",
					Kind:       "NamespacedGeneratingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "generate-cm-skipped",
					Namespace: "test-ns",
				},
				Spec: policiesv1beta1.GeneratingPolicySpec{
					MatchConstraints: matchConstraints,
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "workload-only",
							Expression: `object.metadata.?labels["env"].orValue("") == "prod"`,
						},
					},
					Generation: []policiesv1beta1.Generation{
						{
							Expression: `generator.Apply([{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "gen-child-cm"})}])`,
						},
					},
				},
			},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "trigger-cm",
						"namespace": "test-ns",
						"labels": map[string]interface{}{
							"env": "dev",
						},
					},
				},
			},
			wantRulesCount:   0,
			wantIsSuccessful: true,
			wantPassCount:    0,
			wantErrorCount:   0,
		},
		{
			name: "generation evaluation error reports rule error",
			policy: &policiesv1beta1.NamespacedGeneratingPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policies.kyverno.io/v1beta1",
					Kind:       "NamespacedGeneratingPolicy",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "generate-cm-error",
					Namespace: "test-ns",
				},
				Spec: policiesv1beta1.GeneratingPolicySpec{
					MatchConstraints: matchConstraints,
					Generation: []policiesv1beta1.Generation{
						{
							Expression: `generator.Apply([{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": string(1 / 0)})}])`,
						},
					},
				},
			},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "trigger-cm",
						"namespace": "test-ns",
					},
				},
			},
			wantRulesCount:        1,
			wantRuleStatus:        engineapi.RuleStatusError,
			wantIsSuccessful:      false,
			wantGenResourcesCount: 0,
			wantPassCount:         0,
			wantErrorCount:        1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc := &ResultCounts{}
			processor := PolicyProcessor{
				Store:              &store.Store{},
				GeneratingPolicies: []policiesv1beta1.GeneratingPolicyLike{tc.policy},
				Resource:           tc.resource,
				Variables:          &variables.Variables{},
				Rc:                 rc,
				Out:                io.Discard,
			}

			responses, err := processor.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Equal(t, len(responses), 1)
			assert.Equal(t, responses[0].Policy().GetName(), tc.policy.Name)
			assert.Equal(t, responses[0].Policy().GetNamespace(), tc.policy.Namespace)
			assert.Equal(t, responses[0].Policy().GetKind(), "NamespacedGeneratingPolicy")
			assert.Assert(t, responses[0].Policy().IsNamespaced())
			assert.Equal(t, responses[0].IsSuccessful(), tc.wantIsSuccessful)
			assert.Equal(t, len(responses[0].PolicyResponse.Rules), tc.wantRulesCount)

			if tc.wantRulesCount > 0 {
				assert.Equal(t, responses[0].PolicyResponse.Rules[0].Status(), tc.wantRuleStatus)
				gen := responses[0].PolicyResponse.Rules[0].GeneratedResources()
				assert.Equal(t, len(gen), tc.wantGenResourcesCount)
				if tc.wantGenResourcesCount > 0 {
					assert.Equal(t, gen[0].GetName(), tc.wantGenResourceName)
				}
			}

			assert.Equal(t, rc.Pass, tc.wantPassCount)
			assert.Equal(t, rc.Error, tc.wantErrorCount)
		})
	}
}
