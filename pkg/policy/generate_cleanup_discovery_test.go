package policy

import (
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func cleanupPolicyPattern(policy *kyvernov1.ClusterPolicy, mode, apiVersion, kind string, foreach bool) {
	pattern := policy.Spec.Rules[0].Generation.GeneratePattern
	pattern.APIVersion, pattern.Kind = apiVersion, kind
	switch mode {
	case "data":
		pattern.Clone = kyvernov1.CloneFrom{}
		pattern.SetData(map[string]interface{}{"data": map[string]interface{}{"value": "generated"}})
	case "cloneList":
		gvk := kind
		if apiVersion != "" {
			gvk = apiVersion + "/" + kind
		}
		pattern = kyvernov1.GeneratePattern{CloneList: kyvernov1.CloneList{
			Namespace: "default", Kinds: []string{gvk},
		}}
	}
	if foreach {
		policy.Spec.Rules[0].Generation.GeneratePattern = kyvernov1.GeneratePattern{}
		policy.Spec.Rules[0].Generation.ForEachGeneration = []kyvernov1.ForEachGeneration{{
			List: "request.object.spec.sources", GeneratePattern: pattern,
		}}
	} else {
		policy.Spec.Rules[0].Generation.GeneratePattern = pattern
	}
}

func TestCreateURForDownstreamDeletion_DiscoveryVersions(t *testing.T) {
	t.Parallel()
	versions := []struct {
		name, apiVersion, resolvedVersion, kind string
	}{
		{"core omitted", "", "v1", "ConfigMap"},
		{"core explicit", "v1", "v1", "ConfigMap"},
		{"custom preferred", "", "example.test/v2", "Widget"},
		{"custom explicit nonpreferred", "example.test/v1", "example.test/v1", "Widget"},
	}
	for _, version := range versions {
		for _, mode := range []string{"data", "clone", "cloneList"} {
			for _, foreach := range []bool{false, true} {
				name := version.name + "/" + mode
				if foreach {
					name += "/foreach"
				}
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					policy := buildClonePolicy(false)
					cleanupPolicyPattern(policy, mode, version.apiVersion, version.kind, foreach)
					downstream := buildDownstreamForRule()
					downstream.SetAPIVersion(version.resolvedVersion)
					downstream.SetKind(version.kind)
					store := signPolicyDownstream(t, policy, downstream)
					unsigned := downstream.DeepCopy()
					unsigned.SetName("unsigned-target")
					unsigned.SetUID("unsigned-uid")
					unsigned.SetAnnotations(nil)
					copied := downstream.DeepCopy()
					copied.SetName("copied-stamp-target")
					copied.SetUID("copied-uid")
					client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, downstream, unsigned, copied)
					require.NoError(t, err)
					discovery := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
						{Group: "example.test", Version: "v1", Resource: "widgets"},
						{Group: "example.test", Version: "v2", Resource: "widgets"},
					})
					discovery.SetPreferredVersion("example.test", "v2")
					client.SetDiscovery(discovery)
					generator := &captureURGenerator{}
					controller := &policyController{
						client: client, provenance: store, urGenerator: generator, log: logging.WithName("policy-cleanup-test"),
					}
					updated := policy.DeepCopy()
					updated.Spec.Rules = nil
					deleted, ok, _ := ruleChange(policy, updated)
					require.True(t, ok)
					// Rule removal and policy deletion must retain the same concrete identities.
					for _, subject := range []kyvernov1.PolicyInterface{deleted, policy} {
						generator.captured = nil
						require.NoError(t, controller.createURForDownstreamDeletion(subject))
						require.Len(t, generator.captured, 1)
						ur := generator.captured[0]
						assert.Equal(t, string(policy.UID), ur.Annotations[provenance.CleanupPolicyUIDAnnotation])
						require.Len(t, ur.Spec.RuleContext, 1)
						assert.True(t, ur.Spec.RuleContext[0].DeleteDownstream)
						assert.EqualValues(t, "trigger-uid", ur.Spec.RuleContext[0].Trigger.UID)
						require.Equal(t, []kyvernov1.ResourceSpec{common.ResourceSpecFromUnstructured(*downstream)}, ur.Status.GeneratedResources)
					}
					gv, err := schema.ParseGroupVersion(version.resolvedVersion)
					require.NoError(t, err)
					actions := client.GetDynamicInterface().(*dynamicfake.FakeDynamicClient).Actions()
					require.Len(t, actions, 2)
					for _, action := range actions {
						assert.Equal(t, "list", action.GetVerb())
						assert.Equal(t, gv.WithResource(strings.ToLower(version.kind)+"s"), action.GetResource())
					}
				})
			}
		}
	}
}

func TestCreateURForDownstreamDeletion_DiscoveryFailure(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"Missing", "Widget", "{{ request.object.kind }}"} {
		for _, mode := range []string{"clone", "cloneList"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				t.Parallel()
				policy := buildClonePolicy(false)
				cleanupPolicyPattern(policy, mode, "", kind, true)
				client := dclient.NewEmptyFakeClient()
				client.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
					{Group: "one.test", Version: "v1", Resource: "widgets"},
					{Group: "two.test", Version: "v1", Resource: "widgets"},
				}))
				generator := &captureURGenerator{}
				controller := &policyController{client: client, urGenerator: generator, log: logging.WithName("policy-cleanup-test")}
				require.ErrorContains(t, controller.createURForDownstreamDeletion(policy), "resolve downstream kind")
				assert.Empty(t, generator.captured)
				assert.Empty(t, client.GetDynamicInterface().(*dynamicfake.FakeDynamicClient).Actions(), "discovery failures must not list an unresolved resource")
			})
		}
	}
}
