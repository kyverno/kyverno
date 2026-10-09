package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildURForGenerateRuleChangesRequiresProvenance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*kyvernov1.ClusterPolicy, *unstructured.Unstructured)
		valid  bool
	}{
		{"authenticated downstream", nil, true},
		{"same tuple preplant", func(_ *kyvernov1.ClusterPolicy, obj *unstructured.Unstructured) { obj.SetAnnotations(nil) }, false},
		{"copied stamp replacement UID", func(_ *kyvernov1.ClusterPolicy, obj *unstructured.Unstructured) { obj.SetUID("replacement-uid") }, false},
		{"forged trigger", func(_ *kyvernov1.ClusterPolicy, obj *unstructured.Unstructured) {
			labels := obj.GetLabels()
			labels[common.GenerateTriggerUIDLabel] = "attacker-trigger"
			obj.SetLabels(labels)
		}, false},
		{"recreated policy", func(policy *kyvernov1.ClusterPolicy, _ *unstructured.Unstructured) { policy.UID = "replacement-policy" }, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			policy := buildClonePolicy(false)
			downstream := buildDownstreamForRule()
			store := signPolicyDownstream(t, policy, downstream)
			if test.change != nil {
				test.change(policy, downstream)
			}
			client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, downstream)
			require.NoError(t, err)
			client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
			controller := &policyController{client: client, provenance: store, log: logging.WithName("policy-provenance-test")}
			for _, policyDeletion := range []bool{false, true} {
				ur := newGenerateUR(engineapi.NewKyvernoPolicy(policy))
				result, err := controller.buildURForGenerateRuleChanges(policy, ur, policy.Spec.Rules[0].Name, policy.Spec.Rules[0].Generation.GeneratePattern, policyDeletion, policyDeletion)
				require.NoError(t, err)
				if !test.valid {
					assert.Empty(t, result.Spec.RuleContext)
					assert.Empty(t, result.Status.GeneratedResources)
					continue
				}
				require.Len(t, result.Spec.RuleContext, 1)
				assert.EqualValues(t, "trigger-uid", result.Spec.RuleContext[0].Trigger.UID)
				if policyDeletion {
					require.Len(t, result.Status.GeneratedResources, 1)
					assert.Equal(t, downstream.GetUID(), result.Status.GeneratedResources[0].UID)
				} else {
					assert.Empty(t, result.Status.GeneratedResources)
				}
			}
		})
	}
}

func TestBuildURForGenerateRuleChangesMissingKeyFailsClosed(t *testing.T) {
	t.Parallel()
	policy := buildClonePolicy(false)
	downstream := buildDownstreamForRule()
	signPolicyDownstream(t, policy, downstream)
	client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, downstream)
	require.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	controller := &policyController{client: client, provenance: provenance.NewStore(fake.NewClientset()), log: logging.WithName("policy-provenance-test")}
	ur := newGenerateUR(engineapi.NewKyvernoPolicy(policy))
	result, err := controller.buildURForGenerateRuleChanges(policy, ur, policy.Spec.Rules[0].Name, policy.Spec.Rules[0].Generation.GeneratePattern, false, false)
	require.Error(t, err)
	assert.Empty(t, result.Spec.RuleContext)
	assert.Empty(t, result.Status.GeneratedResources)
}

func TestUnlabelDownstreamRequiresProvenance(t *testing.T) {
	t.Parallel()
	for _, valid := range []bool{false, true} {
		name := "unsigned preplant stays untouched"
		if valid {
			name = "authenticated previous target is released"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			oldPolicy := buildClonePolicy(false)
			downstream := buildDownstreamForRule()
			store := signPolicyDownstream(t, oldPolicy, downstream)
			if !valid {
				downstream.SetAnnotations(nil)
			}
			client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, downstream)
			require.NoError(t, err)
			client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
			controller := &policyController{client: client, provenance: store, log: logging.WithName("policy-provenance-test")}
			newPolicy := oldPolicy.DeepCopy()
			newPolicy.Spec.Rules[0].Generation.Name = "new-target"
			_, _, selector := ruleChange(oldPolicy, newPolicy)
			controller.unlabelDownstream(selector)
			result, err := client.GetResource(t.Context(), downstream.GetAPIVersion(), downstream.GetKind(), downstream.GetNamespace(), downstream.GetName())
			require.NoError(t, err)
			if valid {
				assert.NotContains(t, result.GetLabels(), common.GeneratePolicyLabel)
				assert.NotContains(t, result.GetLabels(), common.GeneratePolicyNamespaceLabel)
				assert.NotContains(t, result.GetLabels(), common.GenerateRuleLabel)
				assert.NotContains(t, result.GetAnnotations(), provenance.Annotation)
			} else {
				assert.Equal(t, downstream, result)
			}
		})
	}
}

func signPolicyDownstream(t *testing.T, policy kyvernov1.PolicyInterface, obj *unstructured.Unstructured) *provenance.Store {
	t.Helper()
	client := fake.NewClientset()
	require.NoError(t, provenance.EnsureKey(t.Context(), client.CoreV1().Secrets(config.KyvernoNamespace())))
	store := provenance.NewStore(client)
	stamp, err := store.Sign(t.Context(), policy, obj)
	require.NoError(t, err)
	obj.SetAnnotations(map[string]string{provenance.Annotation: stamp})
	return store
}
