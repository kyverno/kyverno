//go:build integration

package gpol_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	gpol "github.com/kyverno/kyverno/pkg/webhooks/resource/gpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	testEnv     *framework.TestEnv
	gpolLister  policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister
	cancelInf   context.CancelFunc
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
	)
	if err != nil {
		panic(err)
	}

	// Start informer factory for gpol listers — mirrors production wiring.
	infCtx, cancel := context.WithCancel(context.Background())
	cancelInf = cancel
	gpolLister, ngpolLister = framework.NewGpolListers(infCtx, testEnv.KyvernoClient)

	if err := testEnv.Start(); err != nil {
		cancel()
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	cancelInf()
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
