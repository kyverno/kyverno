package policy

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type countingListClient struct {
	dclient.Interface
	lists int
}

func (c *countingListClient) ListResource(ctx context.Context, apiVersion, kind, namespace string, selector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if kind == "ConfigMap" && selector == nil {
		c.lists++
	}
	return c.Interface.ListResource(ctx, apiVersion, kind, namespace, selector)
}

func TestBuildURForGenerateRuleChangesResolvesTriggerNames(t *testing.T) {
	t.Parallel()
	policy := buildClonePolicy(false)
	var objects []runtime.Object
	var downstreams []*unstructured.Unstructured
	for _, name := range []string{"a", "b"} {
		trigger := &unstructured.Unstructured{}
		trigger.SetAPIVersion("v1")
		trigger.SetKind("ConfigMap")
		trigger.SetNamespace("default")
		trigger.SetName("trigger-" + name)
		trigger.SetUID(types.UID("trigger-uid-" + name))

		downstream := buildDownstreamForRule()
		downstream.SetName("target-" + name)
		downstream.SetUID(types.UID("downstream-uid-" + name))
		labels := downstream.GetLabels()
		delete(labels, common.GenerateTriggerNameLabel)
		labels[common.GenerateTriggerKindLabel] = "ConfigMap"
		labels[common.GenerateTriggerNSLabel] = "default"
		labels[common.GenerateTriggerUIDLabel] = string(trigger.GetUID())
		downstream.SetLabels(labels)
		downstreams = append(downstreams, downstream)
		objects = append(objects, trigger)
	}
	store := signPolicyDownstream(t, policy, downstreams[0])
	stamp, err := store.Sign(t.Context(), policy, downstreams[1])
	require.NoError(t, err)
	downstreams[1].SetAnnotations(map[string]string{provenance.Annotation: stamp})
	for _, d := range downstreams {
		objects = append(objects, d)
	}

	fakeClient, err := dclient.NewFakeClient(runtime.NewScheme(), nil, objects...)
	require.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	client := &countingListClient{Interface: fakeClient}
	controller := &policyController{client: client, provenance: store, log: logging.WithName("policy-trigger-name-test")}

	ur := newGenerateUR(engineapi.NewKyvernoPolicy(policy))
	result, err := controller.buildURForGenerateRuleChanges(policy, ur, policy.Spec.Rules[0].Name, policy.Spec.Rules[0].Generation.GeneratePattern, false, false)
	require.NoError(t, err)

	names := map[types.UID]string{}
	for _, rc := range result.Spec.RuleContext {
		names[rc.Trigger.UID] = rc.Trigger.Name
	}
	assert.Equal(t, map[types.UID]string{"trigger-uid-a": "trigger-a", "trigger-uid-b": "trigger-b"}, names)
	assert.Equal(t, 1, client.lists)
}
