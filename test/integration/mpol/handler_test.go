//go:build integration

package mpol_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	mpol "github.com/kyverno/kyverno/pkg/webhooks/resource/mpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gomodules.xyz/jsonpatch/v2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testEnv  *framework.TestEnv
	engine   mpolengine.Engine
	provider mpolengine.Provider
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
	)
	if err != nil {
		panic(err)
	}

	engine, provider, err = framework.NewMpolEngine(context.Background(), testEnv.Mgr, testEnv.KubeClient, testEnv.ContextProvider)
	if err != nil {
		testEnv.Stop()
		panic(err)
	}

	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}

// waitForPolicyReady waits until the provider has compiled at least count policies.
func waitForPolicyReady(t *testing.T, count int) {
	t.Helper()
	ctx := context.Background()
	require.Eventually(t, func() bool {
		return len(provider.Fetch(ctx, false)) >= count
	}, 5*time.Second, 100*time.Millisecond, "policies not reconciled in time")
}

// waitForPolicyGone waits until the provider has no compiled policies.
func waitForPolicyGone(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	require.Eventually(t, func() bool {
		return len(provider.Fetch(ctx, false)) == 0
	}, 5*time.Second, 100*time.Millisecond, "policies not cleaned up in time")
}

// createPolicyWithCleanup creates a MutatingPolicy and registers cleanup.
func createPolicyWithCleanup(t *testing.T, policy *policiesv1beta1.MutatingPolicy) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
}

// decodePatches parses the JSON patch bytes from an admission response.
func decodePatches(t *testing.T, patchBytes []byte) []jsonpatch.JsonPatchOperation {
	t.Helper()
	var patches []jsonpatch.JsonPatchOperation
	require.NoError(t, json.Unmarshal(patchBytes, &patches))
	return patches
}

// findPatch returns the first patch operation matching the given path.
func findPatch(patches []jsonpatch.JsonPatchOperation, path string) *jsonpatch.JsonPatchOperation {
	for _, p := range patches {
		if p.Path == path {
			return &p
		}
	}
	return nil
}

func TestMutate_JSONPatch_AddsLabel(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-env-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/env", value: "production"}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "add-env-label")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("my-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "my-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "mutation should allow the resource")
	require.NotNil(t, resp.Patch, "mutation should produce patches")

	patches := decodePatches(t, resp.Patch)
	p := findPatch(patches, "/metadata/labels/env")
	require.NotNil(t, p, "patch for /metadata/labels/env should exist")
	assert.Equal(t, "add", p.Operation)
	assert.Equal(t, "production", p.Value)
}

func TestMutate_JSONPatch_AllowsWithoutChange(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-label-if-missing"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "missing-env-label",
				Expression: "!has(object.metadata.labels) || !('env' in object.metadata.labels)",
			}},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/env", value: "default"}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	// Pod already has the env label — match condition should skip it
	ctx := framework.ContextWithPolicies(context.Background(), "add-label-if-missing")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("existing-label-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "existing-label-pod", "namespace": "default", "labels": {"env": "staging"}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "already-compliant pod should be allowed")
	assert.Nil(t, resp.Patch, "no mutation should be applied when match condition skips")
}

func TestMutate_MatchCondition_SkipsNonMatching(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-only-prod-ns"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "only-production",
				Expression: "object.metadata.namespace == 'production'",
			}},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/mutated", value: "true"}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	// Pod is in "default" namespace, not "production" — should be skipped
	ctx := framework.ContextWithPolicies(context.Background(), "mutate-only-prod-ns")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("my-app", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "my-app", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "pod in non-matching namespace should pass through")
	assert.Nil(t, resp.Patch, "no mutation should apply when match condition is not met")
}

func TestMutate_EventsGenerated_OnMutation(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-team-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/team", value: "platform"}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "add-team-label")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("event-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "event-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	// Wait for the async audit goroutine to generate events
	time.Sleep(200 * time.Millisecond)

	assert.True(t, resp.Allowed, "mutation should allow the resource")
	assert.NotEmpty(t, eventGen.Events, "mutation should generate events")
}

func TestMutate_CELVariables_UsedInMutation(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-computed-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{{
				Name:       "prefix",
				Expression: "'kyverno-'",
			}},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/managed-by", value: variables.prefix + "controller"}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "add-computed-label")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("var-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "var-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "mutation with variables should allow the resource")
	require.NotNil(t, resp.Patch, "mutation with variables should produce patches")

	patches := decodePatches(t, resp.Patch)
	p := findPatch(patches, "/metadata/labels/managed-by")
	require.NotNil(t, p, "patch for /metadata/labels/managed-by should exist")
	assert.Equal(t, "kyverno-controller", p.Value)
}

func TestMutate_MultipleMutations_ChainedCorrectly(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-multiple-labels"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
						Expression: `[JSONPatch{op: "add", path: "/metadata/labels/tier", value: "frontend"}]`,
					},
				},
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
						Expression: `[JSONPatch{op: "add", path: "/metadata/labels/cost-center", value: "engineering"}]`,
					},
				},
			},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "add-multiple-labels")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("multi-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "multi-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "multiple mutations should allow the resource")
	require.NotNil(t, resp.Patch, "multiple mutations should produce patches")

	patches := decodePatches(t, resp.Patch)

	tierPatch := findPatch(patches, "/metadata/labels/tier")
	require.NotNil(t, tierPatch, "patch for /metadata/labels/tier should exist")
	assert.Equal(t, "frontend", tierPatch.Value)

	costPatch := findPatch(patches, "/metadata/labels/cost-center")
	require.NotNil(t, costPatch, "patch for /metadata/labels/cost-center should exist")
	assert.Equal(t, "engineering", costPatch.Value)
}
