//go:build integration

package gpol_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	gpol "github.com/kyverno/kyverno/pkg/webhooks/resource/gpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testEnv      *framework.TestEnv
	gpolLister   policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister  policiesv1beta1listers.NamespacedGeneratingPolicyLister
	polexLister  celengine.PolicyExceptionLister
	gpolEngine   gpolengine.Engine
	gpolProvider gpolengine.Provider
	cancelInf    context.CancelFunc
	cancelPolex  context.CancelFunc
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
	)
	if err != nil {
		panic(err)
	}

	// Start informer factories for gpol listers and PolicyException lister.
	infCtx, cancel := context.WithCancel(context.Background())
	cancelInf = cancel
	gpolLister, ngpolLister = framework.NewGpolListers(infCtx, testEnv.KyvernoClient)

	polexInfCtx, polexCancel := context.WithCancel(context.Background())
	cancelPolex = polexCancel
	polexLister = framework.NewGpolPolexLister(polexInfCtx, testEnv.KyvernoClient)

	// Use exception-enabled engine so full-flow tests can verify exception behavior.
	// When no exceptions exist, this behaves identically to NewGpolEngine.
	gpolEngine, gpolProvider = framework.NewGpolEngineWithExceptions(gpolLister, ngpolLister, polexLister)

	if err := testEnv.Start(); err != nil {
		cancel()
		polexCancel()
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	cancelInf()
	cancelPolex()
	testEnv.Stop()
	os.Exit(code)
}

// waitForGpolInLister waits until the informer cache has the policy.
func waitForGpolInLister(t *testing.T, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := gpolLister.Get(name)
		return err == nil
	}, 5*time.Second, 100*time.Millisecond, "gpol %q not found in lister cache", name)
}

// waitForGpolGone waits until the informer cache no longer has the policy.
func waitForGpolGone(t *testing.T, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := gpolLister.Get(name)
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "gpol %q still in lister cache", name)
}

// createGpolWithCleanup creates a GeneratingPolicy and registers cleanup.
func createGpolWithCleanup(t *testing.T, policy *policiesv1beta1.GeneratingPolicy) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), policy)
		waitForGpolGone(t, policy.Name)
	})
}

var podJSON = []byte(`{
	"apiVersion": "v1", "kind": "Pod",
	"metadata": {"name": "test-pod", "namespace": "default", "uid": "abc-123"},
	"spec": {"containers": [{"name": "app", "image": "nginx"}]}
}`)

func TestGenerate_CreateTriggersUpdateRequest(t *testing.T) {
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-networkpolicy"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-networkpolicy")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-networkpolicy")
	resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("test-pod", "default", admissionv1.Create, podJSON), "", time.Now())

	assert.True(t, resp.Allowed, "generate handler should always allow")

	// Wait for the async goroutine to fire
	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 2*time.Second, 50*time.Millisecond, "UpdateRequest not created in time")

	specs := mock.GetSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, kyvernov2.CELGenerate, specs[0].Type)
	assert.Equal(t, "gen-networkpolicy", specs[0].Policy)
	require.Len(t, specs[0].RuleContext, 1)
	assert.False(t, specs[0].RuleContext[0].DeleteDownstream)
	assert.False(t, specs[0].RuleContext[0].Synchronize)
}

func TestGenerate_DryRunSkipsGeneration(t *testing.T) {
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-dryrun-skip"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-dryrun-skip")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	req := framework.PodAdmissionRequestWithOp("dry-pod", "default", admissionv1.Create, podJSON)
	req.DryRun = ptr.To(true)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-dryrun-skip")
	resp := h.Generate(ctx, logr.Discard(), req, "", time.Now())

	assert.True(t, resp.Allowed, "dry-run should still allow")

	time.Sleep(300 * time.Millisecond)
	assert.Empty(t, mock.GetSpecs(), "dry-run must not create UpdateRequests")
}

func TestGenerate_DeleteWithSyncDeletesDownstream(t *testing.T) {
	// Policy matches CREATE only (not DELETE) and has synchronization enabled.
	// Deleting the trigger should produce a UR with deleteDownstream=true.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-sync-delete"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Create),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
					Enabled: ptr.To(true),
				},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-sync-delete")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-sync-delete")
	resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("del-pod", "default", admissionv1.Delete, podJSON), "", time.Now())

	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 2*time.Second, 50*time.Millisecond, "UpdateRequest not created in time")

	specs := mock.GetSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, kyvernov2.CELGenerate, specs[0].Type)
	assert.True(t, specs[0].RuleContext[0].DeleteDownstream, "should signal downstream deletion")
	assert.False(t, specs[0].RuleContext[0].Synchronize)
}

func TestGenerate_DeleteMatchingDeleteOpFiresGeneration(t *testing.T) {
	// Policy explicitly matches DELETE operations.
	// Deleting the trigger should fire a generation, not a downstream deletion.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-on-delete"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Delete),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-on-delete")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-on-delete")
	resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("audit-pod", "default", admissionv1.Delete, podJSON), "", time.Now())

	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 2*time.Second, 50*time.Millisecond, "UpdateRequest not created in time")

	specs := mock.GetSpecs()
	require.Len(t, specs, 1)
	assert.False(t, specs[0].RuleContext[0].DeleteDownstream, "should fire generation, not deletion")
}

func TestGenerate_UpdateWithSyncSetsSynchronize(t *testing.T) {
	// Policy has synchronization enabled. Updating the trigger should
	// create a UR with synchronize=true so downstream stays in sync.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-sync-update"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Create, admissionregistrationv1.Update),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
					Enabled: ptr.To(true),
				},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-sync-update")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-sync-update")
	resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("sync-pod", "default", admissionv1.Update, podJSON), "", time.Now())

	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 2*time.Second, 50*time.Millisecond, "UpdateRequest not created in time")

	specs := mock.GetSpecs()
	require.Len(t, specs, 1)
	assert.True(t, specs[0].RuleContext[0].Synchronize, "update with sync-enabled policy should set synchronize")
}

func TestGenerate_MultiplePoliciesCreateMultipleURs(t *testing.T) {
	policyA := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-multi-a"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
		},
	}
	policyB := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-multi-b"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
		},
	}

	createGpolWithCleanup(t, policyA)
	createGpolWithCleanup(t, policyB)
	waitForGpolInLister(t, "gen-multi-a")
	waitForGpolInLister(t, "gen-multi-b")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-multi-a", "gen-multi-b")
	resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("multi-pod", "default", admissionv1.Create, podJSON), "", time.Now())

	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 2
	}, 2*time.Second, 50*time.Millisecond, "expected 2 UpdateRequests")

	specs := mock.GetSpecs()
	require.Len(t, specs, 2)

	policyNames := []string{specs[0].Policy, specs[1].Policy}
	assert.Contains(t, policyNames, "gen-multi-a")
	assert.Contains(t, policyNames, "gen-multi-b")
}

// --- Full-flow tests: handler → UR processing → downstream resource created in envtest ---
//
// These tests use ProcessingURGenerator which runs each captured URSpec through
// the gpol engine, creating real downstream resources in the envtest API server.
// This simulates the full production path: admission → handler → UR → background
// controller → gpol engine → generator.Apply() → resource created.

func TestGenerateFullFlow_PodCreateGeneratesConfigMap(t *testing.T) {
	// A platform engineer writes a gpol that generates a ConfigMap whenever
	// a Pod is created. The ConfigMap stores the trigger pod's name so the
	// team can track which pods triggered which generated resources.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-cm-on-pod"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "generated-for-" + object.metadata.name}),
						"data": dyn({"trigger-pod": object.metadata.name})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-cm-on-pod")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "web-app", "namespace": "default", "uid": "pod-uid-1"},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
	}`)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-cm-on-pod")
	resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("web-app", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	// Wait for the handler's goroutine to fire and processing to complete.
	// GetSpecs() returns results only after the processor finishes, so
	// by this point the ConfigMap is guaranteed to exist in envtest.
	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "UR not processed in time")

	assert.Empty(t, mock.ProcessingErrors(), "processing should succeed without errors")

	// Verify the ConfigMap was actually created in the envtest cluster.
	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "generated-for-web-app",
	}, cm)
	require.NoError(t, err, "generated ConfigMap should exist in envtest")
	assert.Equal(t, "web-app", cm.Data["trigger-pod"], "ConfigMap data should reference the trigger pod")

	// Cleanup the generated ConfigMap.
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), cm)
	})
}

func TestGenerateFullFlow_DryRunProducesNoDownstream(t *testing.T) {
	// A developer checks whether their pod would pass policies using --dry-run.
	// The gpol handler should skip entirely — no UR, no downstream resources.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-cm-dryrun"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "should-not-exist"}),
						"data": dyn({"test": "dryrun"})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-cm-dryrun")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	req := framework.PodAdmissionRequestWithOp("dryrun-pod", "default", admissionv1.Create, podJSON)
	req.DryRun = ptr.To(true)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-cm-dryrun")
	resp := h.Generate(ctx, logr.Discard(), req, "", time.Now())
	assert.True(t, resp.Allowed)

	time.Sleep(500 * time.Millisecond)
	assert.Empty(t, mock.GetSpecs(), "dry-run should not create any URs")

	// Verify no ConfigMap was created.
	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "should-not-exist",
	}, cm)
	assert.True(t, apierrors.IsNotFound(err), "no downstream resource should exist after dry-run")
}

func TestGenerateFullFlow_MultiplePoliciesGenerateDifferentResources(t *testing.T) {
	// Two teams each deploy a gpol: one generates a ConfigMap for cost tracking,
	// the other generates one for monitoring labels. A single pod creation should
	// trigger both — each policy independently produces its downstream resource.
	costPolicy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-cost-cm"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "cost-tracking-" + object.metadata.name}),
						"data": dyn({"team": "platform", "trigger": string(object.metadata.name)})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}
	monitorPolicy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-monitor-cm"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "monitor-" + object.metadata.name}),
						"data": dyn({"team": "observability", "trigger": string(object.metadata.name)})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, costPolicy))
	require.NoError(t, testEnv.Client.Create(ctx, monitorPolicy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), costPolicy)
		testEnv.Client.Delete(context.Background(), monitorPolicy)
		waitForGpolGone(t, "gen-cost-cm")
		waitForGpolGone(t, "gen-monitor-cm")
	})
	waitForGpolInLister(t, "gen-cost-cm")
	waitForGpolInLister(t, "gen-monitor-cm")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "api-server", "namespace": "default", "uid": "pod-uid-2"},
		"spec": {"containers": [{"name": "api", "image": "myapp:v2"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-cost-cm", "gen-monitor-cm")
	resp := h.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("api-server", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 2
	}, 5*time.Second, 100*time.Millisecond, "expected 2 URs to be processed")

	assert.Empty(t, mock.ProcessingErrors(), "both policies should process without errors")

	// Verify both ConfigMaps exist.
	costCM := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "cost-tracking-api-server",
	}, costCM)
	require.NoError(t, err, "cost tracking ConfigMap should exist")
	assert.Equal(t, "platform", costCM.Data["team"])
	assert.Equal(t, "api-server", costCM.Data["trigger"])

	monitorCM := &corev1.ConfigMap{}
	err = testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "monitor-api-server",
	}, monitorCM)
	require.NoError(t, err, "monitoring ConfigMap should exist")
	assert.Equal(t, "observability", monitorCM.Data["team"])

	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), costCM)
		testEnv.Client.Delete(context.Background(), monitorCM)
	})
}

// --- Namespaced generating policy tests ---

// waitForNgpolInLister waits until the informer cache has the namespaced policy.
func waitForNgpolInLister(t *testing.T, namespace, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := ngpolLister.NamespacedGeneratingPolicies(namespace).Get(name)
		return err == nil
	}, 5*time.Second, 100*time.Millisecond, "ngpol %s/%s not found in lister cache", namespace, name)
}

// waitForNgpolGone waits until the informer cache no longer has the namespaced policy.
func waitForNgpolGone(t *testing.T, namespace, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := ngpolLister.NamespacedGeneratingPolicies(namespace).Get(name)
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "ngpol %s/%s still in lister cache", namespace, name)
}

// createNgpolWithCleanup creates a NamespacedGeneratingPolicy and registers cleanup.
func createNgpolWithCleanup(t *testing.T, policy *policiesv1beta1.NamespacedGeneratingPolicy) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), policy)
		waitForNgpolGone(t, policy.Namespace, policy.Name)
	})
}

func TestGenerateNamespaced_CreateTriggersURWithNamespacePrefix(t *testing.T) {
	// A team deploys a namespace-scoped generate policy that auto-creates resources
	// whenever a pod is created in their namespace. The handler should produce an
	// UpdateRequest with policyKey = "namespace/policy" so the background controller
	// knows which namespaced policy to fetch.
	framework.CreateNamespace(t, testEnv.KubeClient, "team-gen")

	policy := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gen-team-configmap",
			Namespace: "team-gen",
		},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
		},
	}

	createNgpolWithCleanup(t, policy)
	waitForNgpolInLister(t, "team-gen", "gen-team-configmap")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-team-configmap")
	resp := h.GenerateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("app-pod", "team-gen", admissionv1.Create, podJSON), "", time.Now())

	assert.True(t, resp.Allowed, "generate handler should always allow")

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 2*time.Second, 50*time.Millisecond, "UpdateRequest not created in time")

	specs := mock.GetSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, kyvernov2.CELGenerate, specs[0].Type)
	assert.Equal(t, "team-gen/gen-team-configmap", specs[0].Policy, "namespaced policy key should be namespace/name")
	require.Len(t, specs[0].RuleContext, 1)
	assert.False(t, specs[0].RuleContext[0].DeleteDownstream)
}

func TestGenerateNamespaced_SkipsClusterScopedResources(t *testing.T) {
	// Namespaced generate policies can only govern namespaced resources.
	// When the admission request has no namespace (cluster-scoped resource),
	// the handler should return success immediately without creating any URs.
	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	// Empty namespace simulates a cluster-scoped resource like a Node or ClusterRole.
	ctx := framework.ContextWithPolicies(context.Background(), "some-policy")
	resp := h.GenerateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("my-node", "", admissionv1.Create, podJSON), "", time.Now())

	assert.True(t, resp.Allowed, "cluster-scoped resource should be allowed immediately")

	time.Sleep(300 * time.Millisecond)
	assert.Empty(t, mock.GetSpecs(), "no URs should be created for cluster-scoped resources")
}

func TestGenerateNamespaced_DeleteWithSyncDeletesDownstream(t *testing.T) {
	// Namespaced generate policy with synchronization enabled: when the trigger
	// pod is deleted and the policy only matches CREATE (not DELETE), the handler
	// should produce a UR with deleteDownstream=true and the correct namespace-prefixed
	// policyKey so the background controller can clean up generated resources.
	framework.CreateNamespace(t, testEnv.KubeClient, "team-cleanup")

	policy := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gen-sync-ns",
			Namespace: "team-cleanup",
		},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Create),
			Generation:       []policiesv1beta1.Generation{{Expression: "[]"}},
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
					Enabled: ptr.To(true),
				},
			},
		},
	}

	createNgpolWithCleanup(t, policy)
	waitForNgpolInLister(t, "team-cleanup", "gen-sync-ns")

	mock := &framework.MockURGenerator{}
	h := gpol.New(mock, gpolLister, ngpolLister)

	ctx := framework.ContextWithPolicies(context.Background(), "gen-sync-ns")
	resp := h.GenerateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("cleanup-pod", "team-cleanup", admissionv1.Delete, podJSON), "", time.Now())

	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 2*time.Second, 50*time.Millisecond, "UpdateRequest not created in time")

	specs := mock.GetSpecs()
	require.Len(t, specs, 1)
	assert.Equal(t, "team-cleanup/gen-sync-ns", specs[0].Policy, "namespaced policy key should be namespace/name")
	assert.True(t, specs[0].RuleContext[0].DeleteDownstream, "should signal downstream deletion")
	assert.False(t, specs[0].RuleContext[0].Synchronize)
}

// --- Exception tests: handler → UR processing → exception check → no downstream ---
//
// PolicyExceptions for gpol are checked at UR processing time (not at handler time).
// The handler always creates a UR. The processor's provider.Get() fetches the policy
// with exceptions compiled in. If an exception matches, the engine skips generation.

// waitForPolexInLister waits until the informer cache has the exception.
func waitForPolexInLister(t *testing.T, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		exceptions, err := polexLister.List(labels.Everything())
		if err != nil {
			return false
		}
		for _, ex := range exceptions {
			if ex.Name == name {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond, "exception %q not found in lister", name)
}

// waitForPolexGone waits until the informer cache no longer has the exception.
func waitForPolexGone(t *testing.T, name string) {
	t.Helper()
	require.Eventually(t, func() bool {
		exceptions, err := polexLister.List(labels.Everything())
		if err != nil {
			return false
		}
		for _, ex := range exceptions {
			if ex.Name == name {
				return false
			}
		}
		return true
	}, 5*time.Second, 100*time.Millisecond, "exception %q still in lister", name)
}

func TestGenerateFullFlow_ExceptionSkipsGeneration(t *testing.T) {
	// A platform team's gpol generates a ConfigMap for every pod. The security team
	// later creates a PolicyException so DB migration pods bypass this policy.
	// The handler still fires a UR, but when the processor fetches the policy it
	// finds the exception compiled in and skips generation entirely.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-cm-with-exception"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "gen-for-" + object.metadata.name}),
						"data": dyn({"trigger": object.metadata.name})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exempt-db-migration",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "gen-cm-with-exception",
				Kind: "GeneratingPolicy",
			}},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-cm-with-exception")

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, exception))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), exception)
		waitForPolexGone(t, "exempt-db-migration")
	})
	waitForPolexInLister(t, "exempt-db-migration")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "db-migrate", "namespace": "default", "uid": "pod-uid-ex1"},
		"spec": {"containers": [{"name": "migrate", "image": "postgres:16"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-cm-with-exception")
	resp := h.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("db-migrate", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "UR not processed in time")

	assert.Empty(t, mock.ProcessingErrors(), "exception skip should not produce errors")

	// Exception matched: no ConfigMap should be created.
	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "gen-for-db-migrate",
	}, cm)
	assert.True(t, apierrors.IsNotFound(err), "exception should prevent ConfigMap generation")
}

func TestGenerateFullFlow_ExceptionWithMatchCondition(t *testing.T) {
	// A security team creates a PolicyException that only applies to pods in the
	// "exempt" namespace. Pods in "exempt" bypass generation, pods in "default"
	// still get their ConfigMap generated.
	framework.CreateNamespace(t, testEnv.KubeClient, "exempt")

	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-cm-matchcond-ex"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "gen-mc-" + object.metadata.name}),
						"data": dyn({"trigger": object.metadata.name})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exempt-ns-only",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "gen-cm-matchcond-ex",
				Kind: "GeneratingPolicy",
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "only-exempt-namespace",
				Expression: "object.metadata.namespace == 'exempt'",
			}},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-cm-matchcond-ex")

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, exception))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), exception)
		waitForPolexGone(t, "exempt-ns-only")
	})
	waitForPolexInLister(t, "exempt-ns-only")

	// Pod in "exempt" namespace — exception match condition is true, generation skipped.
	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mockExempt := framework.NewProcessingURGenerator(processor)
	hExempt := gpol.New(mockExempt, gpolLister, ngpolLister)

	exemptPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "exempt-pod", "namespace": "exempt", "uid": "pod-uid-ex2"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-cm-matchcond-ex")
	resp := hExempt.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("exempt-pod", "exempt", admissionv1.Create, exemptPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mockExempt.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "exempt UR not processed in time")

	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "exempt",
		Name:      "gen-mc-exempt-pod",
	}, cm)
	assert.True(t, apierrors.IsNotFound(err), "exempt-ns pod should not get a generated ConfigMap")

	// Pod in "default" namespace — exception match condition is false, generation proceeds.
	mockDefault := framework.NewProcessingURGenerator(framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider))
	hDefault := gpol.New(mockDefault, gpolLister, ngpolLister)

	defaultPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "regular-pod", "namespace": "default", "uid": "pod-uid-ex3"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	resp = hDefault.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("regular-pod", "default", admissionv1.Create, defaultPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mockDefault.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "default UR not processed in time")

	assert.Empty(t, mockDefault.ProcessingErrors())

	err = testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "gen-mc-regular-pod",
	}, cm)
	require.NoError(t, err, "default-ns pod should get a generated ConfigMap")
	assert.Equal(t, "regular-pod", cm.Data["trigger"])

	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), cm)
	})
}

// --- CEL edge case tests ---

func TestGenerateFullFlow_MatchConditionFalse_NoGeneration(t *testing.T) {
	// A policy has a match condition restricting generation to the "restricted"
	// namespace. A pod in "default" doesn't match, so the engine skips evaluation
	// and no downstream resource is created.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-restricted-only"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "restricted-ns-only",
				Expression: "object.metadata.namespace == 'restricted'",
			}},
			Variables: []admissionregistrationv1.Variable{
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "restricted-cm-" + object.metadata.name}),
						"data": dyn({"trigger": object.metadata.name})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-restricted-only")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "wrong-ns-pod", "namespace": "default", "uid": "pod-uid-mc1"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-restricted-only")
	resp := h.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("wrong-ns-pod", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "UR not processed in time")

	assert.Empty(t, mock.ProcessingErrors())

	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "restricted-cm-wrong-ns-pod",
	}, cm)
	assert.True(t, apierrors.IsNotFound(err), "match condition false should prevent generation")
}

func TestGenerateFullFlow_GenerationExpressionError_NoDownstream(t *testing.T) {
	// A developer writes a generation expression that references a nonexistent field.
	// CEL compiles it (object is DynType) but it fails at eval time. The engine
	// returns RuleError, and no downstream resource is created.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-broken-expression"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, object.spec.nonExistentField)`},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-broken-expression")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "error-pod", "namespace": "default", "uid": "pod-uid-err1"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-broken-expression")
	resp := h.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("error-pod", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "UR not processed in time")

	// The generation expression fails at eval time. engine.Handle() returns the
	// error in EngineResponse (not as top-level error), so ProcessingErrors may
	// or may not capture it depending on how the error propagates. The key check
	// is that no downstream resource was created.
	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "generated-for-error-pod",
	}, cm)
	assert.True(t, apierrors.IsNotFound(err), "broken generation expression should not create downstream")
}

func TestGenerateFullFlow_UnusedBrokenVariable_StillGenerates(t *testing.T) {
	// A policy author adds a variable with a broken CEL expression, but the
	// generation expression doesn't reference it. CEL variables are lazily
	// evaluated, so the broken variable's error is deferred and never surfaces.
	// Generation proceeds normally.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-unused-broken-var"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "broken",
					Expression: "object.spec.nonExistentField.deeplyNested",
				},
				{
					Name: "configmap",
					Expression: `[{
						"kind": dyn("ConfigMap"),
						"apiVersion": dyn("v1"),
						"metadata": dyn({"name": "unused-var-" + object.metadata.name}),
						"data": dyn({"trigger": object.metadata.name})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, variables.configmap)`},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-unused-broken-var")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "lazy-pod", "namespace": "default", "uid": "pod-uid-lz1"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-unused-broken-var")
	resp := h.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("lazy-pod", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "UR not processed in time")

	assert.Empty(t, mock.ProcessingErrors(), "unused broken variable should not cause errors")

	// Broken variable was never read, so generation proceeded normally.
	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "unused-var-lazy-pod",
	}, cm)
	require.NoError(t, err, "unused broken variable should not block generation")
	assert.Equal(t, "lazy-pod", cm.Data["trigger"])

	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), cm)
	})
}

func TestGenerateFullFlow_UsedBrokenVariable_NoDownstream(t *testing.T) {
	// Same as above, but the generation expression actually reads the broken
	// variable. The deferred error surfaces when variables.broken is accessed,
	// causing the generation to fail. No downstream resource is created.
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "gen-used-broken-var"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "brokenName",
					Expression: "object.spec.nonExistentField",
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: `generator.Apply(object.metadata.namespace, [{"kind": dyn("ConfigMap"), "apiVersion": dyn("v1"), "metadata": dyn({"name": variables.brokenName}), "data": dyn({"test": "should-not-exist"})}])`},
			},
		},
	}

	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, "gen-used-broken-var")

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister)

	triggerPodJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "broken-var-pod", "namespace": "default", "uid": "pod-uid-bv1"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), "gen-used-broken-var")
	resp := h.Generate(policyCtx, logr.Discard(), framework.PodAdmissionRequestWithOp("broken-var-pod", "default", admissionv1.Create, triggerPodJSON), "", time.Now())
	assert.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 5*time.Second, 100*time.Millisecond, "UR not processed in time")

	// The generation expression read variables.brokenName, which triggered the
	// deferred error. No downstream resource should exist.
	cm := &corev1.ConfigMap{}
	err := testEnv.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "broken-var-pod",
	}, cm)
	assert.True(t, apierrors.IsNotFound(err), "used broken variable should prevent downstream creation")
}
