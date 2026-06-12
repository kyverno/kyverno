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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

	// Use exception-enabled engine for all tests. When no PolicyExceptions exist,
	// behavior is identical to the non-exception engine (ListExceptions returns nil).
	engine, provider, err = framework.NewMpolEngineWithExceptions(context.Background(), testEnv.Mgr, testEnv.KubeClient, testEnv.KyvernoClient, testEnv.ContextProvider)
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

// createNamespacedPolicyWithCleanup creates a NamespacedMutatingPolicy and registers cleanup.
func createNamespacedPolicyWithCleanup(t *testing.T, policy *policiesv1beta1.NamespacedMutatingPolicy) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
}

// createNamespace creates a namespace in envtest and registers cleanup.
func createNamespace(t *testing.T, name string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := testEnv.KubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		testEnv.KubeClient.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
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
	assert.NotEmpty(t, eventGen.GetEvents(), "mutation should generate events")
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

// Test 1: Background controller's service account requests should bypass mutation entirely
// to prevent infinite loops when the controller itself creates or modifies resources.
func TestMutate_BackgroundControllerSkipsMutation(t *testing.T) {
	// The mpol handler short-circuits when UserInfo.Username matches the configured
	// background service account name, returning before engine evaluation. To prove
	// the bypass actually fires (and the test would catch the short-circuit being
	// removed), we register a real mutating policy that would otherwise apply a
	// patch to every Pod, then verify the bypass discriminates: a normal request
	// gets the patch, a background-SA request does not.
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "bg-controller-skip-mutation"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/bg-controller-skip", value: "true"}]`,
				},
			}},
		},
	}
	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	bgSA := "system:serviceaccount:kyverno:kyverno-background-controller"
	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, bgSA, eventGen)

	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "bg-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	ctx := framework.ContextWithPolicies(context.Background(), "bg-controller-skip-mutation")

	t.Run("non-background request produces a patch", func(t *testing.T) {
		resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("bg-pod", "default", podJSON), "", time.Now())
		assert.True(t, resp.Allowed, "non-background requests should still be allowed")
		require.NotNil(t, resp.Patch, "configured mutation should produce a patch for normal requests")

		patches := decodePatches(t, resp.Patch)
		assert.NotNil(t, findPatch(patches, "/metadata/labels/bg-controller-skip"), "expected the policy's label patch")
	})

	t.Run("background controller skips mutation", func(t *testing.T) {
		resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequestWithUsername("bg-pod", "default", bgSA, podJSON), "", time.Now())
		assert.True(t, resp.Allowed, "background controller requests should be allowed")
		assert.Nil(t, resp.Patch, "background controller requests should not produce patches")
	})
}

// Test 2: Dry-run requests should evaluate mutations but never create UpdateRequests,
// honoring the SideEffects: NoneOnDryRun webhook contract.
func TestMutate_DryRunSkipsUpdateRequestCreation(t *testing.T) {
	// Mutate-existing policy: has TargetMatchConstraints and MutateExisting enabled.
	// The handler fires URs for these policies unless it's a dry-run.
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-existing-on-pod"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: *framework.PodMatchRules(),
			},
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: ptr.To(true),
				},
			},
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

	urGen := &framework.MockURGenerator{}
	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, urGen, "", eventGen)

	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "dryrun-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	// Non-dry-run should create a UR (proves the setup works)
	t.Run("non-dry-run creates UR", func(t *testing.T) {
		ctx := framework.ContextWithPolicies(context.Background(), "mutate-existing-on-pod")
		resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("dryrun-pod", "default", podJSON), "", time.Now())

		assert.True(t, resp.Allowed)
		// Wait for the async goroutine that fires URs
		time.Sleep(300 * time.Millisecond)
		assert.NotEmpty(t, urGen.GetSpecs(), "non-dry-run should create update requests")
	})

	// Reset UR generator for the dry-run test
	urGen2 := &framework.MockURGenerator{}
	h2 := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, urGen2, "", eventGen)

	// Dry-run should NOT create URs
	t.Run("dry-run skips UR creation", func(t *testing.T) {
		ctx := framework.ContextWithPolicies(context.Background(), "mutate-existing-on-pod")
		resp := h2.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequestDryRun("dryrun-pod", "default", podJSON), "", time.Now())

		assert.True(t, resp.Allowed)
		time.Sleep(300 * time.Millisecond)
		assert.Empty(t, urGen2.GetSpecs(), "dry-run should not create update requests")
	})
}

// Test 3: When a CEL expression fails at evaluation time and failurePolicy is Fail,
// the handler should reject the admission request.
func TestMutate_FailurePolicy_FailBlocksOnCELError(t *testing.T) {
	failPolicy := admissionregistrationv1.Fail
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "broken-cel-fail"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			FailurePolicy:    &failPolicy,
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					// Accessing a non-existent field on a dyn-typed object compiles OK
					// but fails at evaluation time when the actual Pod has no such field.
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/x", value: object.spec.nonExistentField}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "broken-cel-fail")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("fail-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "fail-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "failurePolicy Fail should reject the request on CEL error")
}

// Test 4: When a CEL expression fails at evaluation time and failurePolicy is Ignore,
// the handler should allow the request and silently skip the mutation.
func TestMutate_FailurePolicy_IgnoreAllowsOnCELError(t *testing.T) {
	ignorePolicy := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "broken-cel-ignore"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			FailurePolicy:    &ignorePolicy,
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/x", value: object.spec.nonExistentField}]`,
				},
			}},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "broken-cel-ignore")
	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("ignore-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "ignore-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "failurePolicy Ignore should allow the request on CEL error")
	assert.Nil(t, resp.Patch, "failed mutation should not produce patches")
}

// Test 5: A NamespacedMutatingPolicy in namespace "team-a" should only mutate
// pods in "team-a", not pods in other namespaces. This is the multi-tenancy isolation
// guarantee — each team's policies are scoped to their namespace.
func TestMutateNamespaced_PolicyOnlyAppliesToItsNamespace(t *testing.T) {
	createNamespace(t, "team-a")
	createNamespace(t, "team-b")

	policy := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team-a-inject-label",
			Namespace: "team-a",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/team", value: "frontend"}]`,
				},
			}},
		},
	}

	createNamespacedPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	// Pod in team-a should get mutated
	t.Run("same namespace gets mutated", func(t *testing.T) {
		ctx := framework.ContextWithPolicies(context.Background(), "team-a-inject-label")
		resp := h.MutateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("frontend-pod", "team-a", []byte(`{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": {"name": "frontend-pod", "namespace": "team-a", "labels": {}},
			"spec": {"containers": [{"name": "app", "image": "nginx"}]}
		}`)), "", time.Now())

		assert.True(t, resp.Allowed)
		require.NotNil(t, resp.Patch, "pod in team-a should be mutated")
		patches := decodePatches(t, resp.Patch)
		p := findPatch(patches, "/metadata/labels/team")
		require.NotNil(t, p)
		assert.Equal(t, "frontend", p.Value)
	})

	// Pod in team-b should NOT get mutated
	t.Run("different namespace not mutated", func(t *testing.T) {
		ctx := framework.ContextWithPolicies(context.Background(), "team-a-inject-label")
		resp := h.MutateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("backend-pod", "team-b", []byte(`{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": {"name": "backend-pod", "namespace": "team-b", "labels": {}},
			"spec": {"containers": [{"name": "app", "image": "nginx"}]}
		}`)), "", time.Now())

		assert.True(t, resp.Allowed)
		assert.Nil(t, resp.Patch, "pod in team-b should not be mutated by team-a policy")
	})
}

// Test 6: MutateNamespaced should immediately return success for cluster-scoped
// resources (namespace == ""), since namespaced policies cannot govern them.
func TestMutateNamespaced_SkipsClusterScopedResources(t *testing.T) {
	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	// Simulate a cluster-scoped resource (empty namespace)
	ctx := framework.ContextWithPolicies(context.Background(), "any-policy")
	resp := h.MutateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("cluster-resource", "", []byte(`{
		"apiVersion": "v1", "kind": "Node",
		"metadata": {"name": "cluster-resource"}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "cluster-scoped resources should pass through MutateNamespaced")
	assert.Nil(t, resp.Patch, "cluster-scoped resources should not be mutated")
}

// Test 7: A PolicyException referencing a MutatingPolicy should cause the mutation
// to be skipped for matching resources. This is the break-glass escape hatch:
// security team enforces policy, but a specific workload gets a temporary exemption.
func TestMutate_PolicyExceptionSkipsMutation(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-runasnonroot"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/security", value: "hardened"}]`,
				},
			}},
		},
	}

	// Exception that exempts the policy
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exempt-db-migration",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: "require-runasnonroot",
				Kind: "MutatingPolicy",
			}},
		},
	}

	// Create exception first, then policy — the provider watches exceptions
	// and re-queues referenced policies on changes.
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, exception))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), exception)
	})

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", eventGen)

	ctxWithPolicies := framework.ContextWithPolicies(context.Background(), "require-runasnonroot")
	resp := h.MutateClustered(ctxWithPolicies, logr.Discard(), framework.PodAdmissionRequest("db-migration-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "db-migration-pod", "namespace": "default", "labels": {}},
		"spec": {"containers": [{"name": "migrate", "image": "postgres:16"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "exception should allow the request")
	assert.Nil(t, resp.Patch, "exception should skip the mutation (no patches applied)")
}
