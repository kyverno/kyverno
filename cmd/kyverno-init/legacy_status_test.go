package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newLegacyPolicy(kind, namespace, name string, conditions ...any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kyverno.io/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name},
	}}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	if conditions != nil {
		obj.Object["status"] = map[string]any{"conditions": conditions}
	}
	return obj
}

func staleReadyCondition() map[string]any {
	return map[string]any{"type": "Ready", "status": "True", "reason": "Succeeded", "message": "Ready", "lastTransitionTime": "2025-01-01T00:00:00Z"}
}

func unsupportedCondition() map[string]any {
	return map[string]any{
		"type": legacyReadyConditionType, "status": string(legacyReadyConditionStatus),
		"reason": legacyReadyConditionReason, "message": legacyReadyConditionMessage,
		"lastTransitionTime": "2025-01-01T00:00:00Z",
	}
}

func newLegacyDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"}: "ClusterPolicyList",
		{Group: "kyverno.io", Version: "v1", Resource: "policies"}:        "PolicyList",
	}, objects...)
}

type statusPatch struct {
	namespace, name, subresource string
	patch                        []byte
}

func recordPatches(client *dynamicfake.FakeDynamicClient) *[]statusPatch {
	var patches []statusPatch
	client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		p := action.(k8stesting.PatchAction)
		patches = append(patches, statusPatch{p.GetNamespace(), p.GetName(), p.GetSubresource(), p.GetPatch()})
		return false, nil, nil
	})
	return &patches
}

func TestMarkLegacyStatuses_ReplacesAllConditions(t *testing.T) {
	t.Parallel()
	client := newLegacyDynamicClient(
		newLegacyPolicy("ClusterPolicy", "", "require-team-label", staleReadyCondition(),
			map[string]any{"type": "Other", "status": "True", "reason": "X", "message": "x", "lastTransitionTime": "2025-01-01T00:00:00Z"}),
		newLegacyPolicy("Policy", "default", "ns-policy", staleReadyCondition()),
		newLegacyPolicy("ClusterPolicy", "", "no-status"),
	)
	patches := recordPatches(client)

	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	require.NoError(t, markLegacyStatuses(context.Background(), client, now))
	require.Len(t, *patches, 3)

	for _, p := range *patches {
		assert.Equal(t, "status", p.subresource, "must only write the status subresource")
		var body map[string]any
		require.NoError(t, json.Unmarshal(p.patch, &body))
		conditions := body["status"].(map[string]any)["conditions"].([]any)
		require.Len(t, conditions, 1, "merge patch must replace the whole conditions list")
		c := conditions[0].(map[string]any)
		assert.Equal(t, "Ready", c["type"])
		assert.Equal(t, "False", c["status"])
		assert.Equal(t, legacyReadyConditionReason, c["reason"])
		assert.Equal(t, legacyReadyConditionMessage, c["message"])
		assert.Equal(t, "2026-11-05T00:00:00Z", c["lastTransitionTime"])
	}

	// The patched object really ends up with only the new condition.
	gvr := schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"}
	obj, err := client.Resource(gvr).Get(context.Background(), "require-team-label", metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, hasLegacyUnsupportedStatus(obj))
}

func TestMarkLegacyStatuses_SkipsAlreadyMarked(t *testing.T) {
	t.Parallel()
	client := newLegacyDynamicClient(
		newLegacyPolicy("ClusterPolicy", "", "done", unsupportedCondition()),
		newLegacyPolicy("Policy", "default", "done", unsupportedCondition()),
	)
	patches := recordPatches(client)

	require.NoError(t, markLegacyStatuses(context.Background(), client, time.Now()))
	assert.Empty(t, *patches)
}

func TestMarkLegacyStatuses_CRDAbsent(t *testing.T) {
	t.Parallel()
	client := newLegacyDynamicClient()
	client.PrependReactor("list", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "")
	})

	require.NoError(t, markLegacyStatuses(context.Background(), client, time.Now()))
}
