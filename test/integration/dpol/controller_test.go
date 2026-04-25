//go:build integration

package dpol_test

import (
	"context"
	"os"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/controllers/deleting"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	testEnv    *framework.TestEnv
	deps       *framework.DpolDeps
	cancelDeps context.CancelFunc
	cancelCtrl context.CancelFunc
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
	)
	if err != nil {
		panic(err)
	}

	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}

	depsCtx, dCancel := context.WithCancel(context.Background())
	cancelDeps = dCancel
	deps = framework.NewDpolDeps(
		depsCtx,
		testEnv.DClient,
		testEnv.KyvernoClient,
		testEnv.KubeClient,
		testEnv.Mgr.GetRESTMapper(),
		testEnv.ContextProvider,
	)

	ctrlCtx, cCancel := context.WithCancel(context.Background())
	cancelCtrl = cCancel
	go deps.Controller.Run(ctrlCtx, deleting.Workers)

	code := m.Run()
	cancelCtrl()
	cancelDeps()
	testEnv.Stop()
	os.Exit(code)
}

// --- helpers ---

// matchRulesFor returns MatchResources targeting the given API group/version/resource.
// Operations is set to OperationAll because the deleting controller is scheduled,
// not admission-driven, so the operation set isn't meaningful at evaluation time.
func matchRulesFor(apiGroup, apiVersion, resource string) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{apiGroup},
					APIVersions: []string{apiVersion},
					Resources:   []string{resource},
				},
			},
		}},
	}
}

// configMapMatchRules returns MatchResources targeting ConfigMaps.
func configMapMatchRules() *admissionregistrationv1.MatchResources {
	return matchRulesFor("", "v1", "configmaps")
}

// labelNamespace updates labels on an existing namespace and waits for the
// namespace lister to reflect the change. The deleting controller's nsResolver
// reads from this lister to populate the `namespaceObject` CEL key, so any test
// that exercises namespaceObject in conditions has to prime the cache.
func labelNamespace(t *testing.T, name string, extraLabels map[string]string) {
	t.Helper()
	ctx := context.Background()
	nsClient := testEnv.KubeClient.CoreV1().Namespaces()

	ns, err := nsClient.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	for k, v := range extraLabels {
		ns.Labels[k] = v
	}
	_, err = nsClient.Update(ctx, ns, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		got, err := deps.NsLister.Get(name)
		if err != nil {
			return false
		}
		for k, v := range extraLabels {
			if got.Labels[k] != v {
				return false
			}
		}
		return true
	}, 5*time.Second, 100*time.Millisecond, "label update on namespace %q should propagate to lister", name)
}

// createResource creates an arbitrary resource in envtest via the dclient and
// registers cleanup. Works for any GVK. See createConfigMap for a sugared wrapper.
func createResource(t *testing.T, gvk schema.GroupVersionKind, obj *unstructured.Unstructured) {
	t.Helper()
	obj.SetGroupVersionKind(gvk)
	apiVersion := gvk.GroupVersion().String()
	_, err := testEnv.DClient.CreateResource(context.Background(), apiVersion, gvk.Kind, obj.GetNamespace(), obj, false)
	require.NoError(t, err)
	t.Cleanup(func() {
		// Best-effort: resource may already be deleted by the controller.
		_ = testEnv.DClient.DeleteResource(context.Background(), apiVersion, gvk.Kind, obj.GetNamespace(), obj.GetName(), false, metav1.DeleteOptions{})
	})
}

// createConfigMap creates a ConfigMap in the given namespace and registers cleanup.
// Thin wrapper over createResource for the common ConfigMap-based test scenarios.
func createConfigMap(t *testing.T, name, namespace string, labels map[string]string) {
	t.Helper()
	obj := &unstructured.Unstructured{}
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(labels)
	_ = unstructured.SetNestedStringMap(obj.Object, map[string]string{"key": "value"}, "data")
	createResource(t, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, obj)
}

// createDpolWithCleanup creates a DeletingPolicy and registers cleanup.
func createDpolWithCleanup(t *testing.T, policy *policiesv1beta1.DeletingPolicy) {
	t.Helper()
	ctx := context.Background()
	_, err := testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies().Create(ctx, policy, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies().Delete(context.Background(), policy.Name, metav1.DeleteOptions{})
	})
}

// createNdpolWithCleanup creates a NamespacedDeletingPolicy and registers cleanup.
func createNdpolWithCleanup(t *testing.T, policy *policiesv1beta1.NamespacedDeletingPolicy) {
	t.Helper()
	ctx := context.Background()
	_, err := testEnv.KyvernoClient.PoliciesV1beta1().NamespacedDeletingPolicies(policy.Namespace).Create(ctx, policy, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = testEnv.KyvernoClient.PoliciesV1beta1().NamespacedDeletingPolicies(policy.Namespace).Delete(context.Background(), policy.Name, metav1.DeleteOptions{})
	})
}

// triggerDpolExecution forces the controller to execute a DeletingPolicy immediately
// by pre-seeding an old LastExecutionTime and bumping the spec generation.
func triggerDpolExecution(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()
	dpolClient := testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies()

	// Wait for informer to see the policy
	require.Eventually(t, func() bool {
		_, err := deps.DpolLister.Get(name)
		return err == nil
	}, 5*time.Second, 100*time.Millisecond, "dpol %q not found in lister", name)

	// Get latest version and set an old LastExecutionTime
	pol, err := dpolClient.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	pol.Status.LastExecutionTime = metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	_, err = dpolClient.UpdateStatus(ctx, pol, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Wait for lister to reflect the status update
	require.Eventually(t, func() bool {
		p, err := deps.DpolLister.Get(name)
		if err != nil {
			return false
		}
		return !p.Status.LastExecutionTime.IsZero()
	}, 5*time.Second, 100*time.Millisecond, "dpol %q status not updated in lister", name)

	// Bump generation by toggling the schedule (semantically equivalent).
	// This causes an informer update event with changed generation,
	// which re-enqueues the key for immediate reconciliation.
	pol, err = dpolClient.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	if pol.Spec.Schedule == "* * * * *" {
		pol.Spec.Schedule = "*/1 * * * *"
	} else {
		pol.Spec.Schedule = "* * * * *"
	}
	_, err = dpolClient.Update(ctx, pol, metav1.UpdateOptions{})
	require.NoError(t, err)
}

// triggerNdpolExecution forces the controller to execute a NamespacedDeletingPolicy immediately.
func triggerNdpolExecution(t *testing.T, namespace, name string) {
	t.Helper()
	ctx := context.Background()
	ndpolClient := testEnv.KyvernoClient.PoliciesV1beta1().NamespacedDeletingPolicies(namespace)

	require.Eventually(t, func() bool {
		_, err := deps.NdpolLister.NamespacedDeletingPolicies(namespace).Get(name)
		return err == nil
	}, 5*time.Second, 100*time.Millisecond, "ndpol %s/%s not found in lister", namespace, name)

	pol, err := ndpolClient.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	pol.Status.LastExecutionTime = metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	_, err = ndpolClient.UpdateStatus(ctx, pol, metav1.UpdateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		p, err := deps.NdpolLister.NamespacedDeletingPolicies(namespace).Get(name)
		if err != nil {
			return false
		}
		return !p.Status.LastExecutionTime.IsZero()
	}, 5*time.Second, 100*time.Millisecond, "ndpol %s/%s status not updated in lister", namespace, name)

	pol, err = ndpolClient.Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	if pol.Spec.Schedule == "* * * * *" {
		pol.Spec.Schedule = "*/1 * * * *"
	} else {
		pol.Spec.Schedule = "* * * * *"
	}
	_, err = ndpolClient.Update(ctx, pol, metav1.UpdateOptions{})
	require.NoError(t, err)
}

// --- tests ---

// Platform engineer sets up a cron-based cleanup to delete stale ConfigMaps.
// All targeted ConfigMaps should be deleted after the schedule fires.
func TestDeletingPolicy_BasicDeletion(t *testing.T) {
	ns := "dpol-basic"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "stale-1", ns, map[string]string{"cleanup": "true"})
	createConfigMap(t, "stale-2", ns, map[string]string{"cleanup": "true"})

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-basic-delete"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: configMapMatchRules(),
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "in-target-namespace",
				Expression: `object.metadata.namespace == "dpol-basic"`,
			}},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// Both ConfigMaps should be deleted by the controller.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "stale-1", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "stale-1 should be deleted")

	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "stale-2", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "stale-2 should be deleted")

	// Events should be generated for each deletion.
	require.Eventually(t, func() bool {
		events := deps.EventCapture.GetEvents()
		successCount := 0
		for _, e := range events {
			if e.Reason == event.PolicyApplied && e.Action == event.ResourceCleanedUp {
				successCount++
			}
		}
		return successCount >= 2
	}, 5*time.Second, 200*time.Millisecond, "expected at least 2 success events")
}

// Security team creates a cleanup policy with CEL conditions that only match
// ConfigMaps labeled cleanup=true. Other ConfigMaps in the same namespace
// should survive. Regression test for kyverno/kyverno#12615 where the
// controller stopped iterating after the first non-matching resource.
func TestDeletingPolicy_PartialMatch(t *testing.T) {
	ns := "dpol-partial"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "match-1", ns, map[string]string{"cleanup": "true"})
	createConfigMap(t, "match-2", ns, map[string]string{"cleanup": "true"})
	createConfigMap(t, "keep-this", ns, map[string]string{"cleanup": "false"})

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-partial-match"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: configMapMatchRules(),
			Conditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "in-namespace",
					Expression: `object.metadata.namespace == "dpol-partial"`,
				},
				{
					Name:       "has-cleanup-label",
					Expression: `object.metadata.labels["cleanup"] == "true"`,
				},
			},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// Matching ConfigMaps deleted.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "match-1", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "match-1 should be deleted")

	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "match-2", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "match-2 should be deleted")

	// Non-matching ConfigMap survives.
	cm, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "keep-this", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "keep-this", cm.Name)
}

// Team-scoped cleanup: a NamespacedDeletingPolicy in ns-a should only
// delete resources within ns-a, not in ns-b.
func TestNamespacedDeletingPolicy_NamespaceIsolation(t *testing.T) {
	nsA := "dpol-ns-a"
	nsB := "dpol-ns-b"
	framework.CreateNamespace(t, testEnv.KubeClient, nsA)
	framework.CreateNamespace(t, testEnv.KubeClient, nsB)
	deps.EventCapture.Clear()

	createConfigMap(t, "target", nsA, map[string]string{"cleanup": "true"})
	createConfigMap(t, "safe", nsB, map[string]string{"cleanup": "true"})

	policy := &policiesv1beta1.NamespacedDeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ndpol-ns-isolation",
			Namespace: nsA,
		},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: configMapMatchRules(),
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "always",
				Expression: "true",
			}},
		},
	}
	createNdpolWithCleanup(t, policy)
	triggerNdpolExecution(t, nsA, policy.Name)

	// ConfigMap in ns-a should be deleted.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(nsA).Get(context.Background(), "target", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "target in ns-a should be deleted")

	// ConfigMap in ns-b should survive.
	cm, err := testEnv.KubeClient.CoreV1().ConfigMaps(nsB).Get(context.Background(), "safe", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "safe", cm.Name)
}

// Platform engineer centralizes an environment value in a dpol variable and
// references it from a CEL condition. Regression test for kyverno/kyverno#15843
// (variables were compiled after conditions, which broke any condition using
// "variables.X"). With that fix in place, this should work.
func TestDeletingPolicy_VariablesInConditions(t *testing.T) {
	ns := "dpol-vars"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "env-test", ns, map[string]string{"env": "test"})
	createConfigMap(t, "env-prod", ns, map[string]string{"env": "prod"})

	// ObjectSelector pre-filters to only ConfigMaps with the "env" label,
	// which keeps kube-root-ca.crt (no "env" label) out of the matched set
	// and avoids a CEL eval error on missing map key.
	rules := configMapMatchRules()
	rules.ObjectSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "env",
			Operator: metav1.LabelSelectorOpExists,
		}},
	}

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-variables-in-conditions"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: rules,
			Variables: []admissionregistrationv1.Variable{{
				Name:       "targetEnv",
				Expression: `"test"`,
			}},
			Conditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "in-namespace",
					Expression: `object.metadata.namespace == "dpol-vars"`,
				},
				{
					Name:       "matches-target-env",
					Expression: `object.metadata.labels["env"] == variables.targetEnv`,
				},
			},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// Only the "test" env ConfigMap should be deleted.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "env-test", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "env-test should be deleted")

	// The "prod" env ConfigMap should survive.
	cm, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "env-prod", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "env-prod", cm.Name)
}

// After the controller executes a policy, it should update the policy's
// Status.LastExecutionTime. This verifies the schedule-requeue loop works
// correctly. Regression test for kyverno/kyverno#10418 where the controller
// stalled after the first deletion cycle.
func TestDeletingPolicy_StatusUpdatedAfterExecution(t *testing.T) {
	ns := "dpol-status"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)

	createConfigMap(t, "to-delete", ns, nil)

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-status-check"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: configMapMatchRules(),
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "in-namespace",
				Expression: `object.metadata.namespace == "dpol-status"`,
			}},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// Wait for the deletion to happen
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "to-delete", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "to-delete should be deleted")

	// Verify that the controller updated the policy's status with a recent execution time.
	// The controller calls updateDeletingPolicyStatus(time.Now()) after deleting() succeeds.
	require.Eventually(t, func() bool {
		pol, err := testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		// After execution, LastExecutionTime should be recent (within the last 30 seconds).
		return !pol.Status.LastExecutionTime.IsZero() &&
			time.Since(pol.Status.LastExecutionTime.Time) < 30*time.Second
	}, 15*time.Second, 200*time.Millisecond, "LastExecutionTime should be updated to a recent time")
}

// Empty conditions list means "match all resources" (no conditions to fail).
// A platform engineer who specifies only MatchConstraints without conditions
// expects all matching resources to be deleted.
func TestDeletingPolicy_EmptyConditionsMatchAll(t *testing.T) {
	ns := "dpol-empty-cond"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "cm-1", ns, nil)
	createConfigMap(t, "cm-2", ns, nil)

	// NamespaceSelector restricts the policy to only this test namespace,
	// preventing cross-namespace deletions while keeping Conditions empty.
	rules := configMapMatchRules()
	rules.NamespaceSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{"kubernetes.io/metadata.name": ns},
	}

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-empty-conditions"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: rules,
			// No conditions — should match all resources that pass MatchConstraints.
			Conditions: nil,
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// Both ConfigMaps should be deleted (empty conditions = match all).
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "cm-1", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "cm-1 should be deleted")

	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "cm-2", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "cm-2 should be deleted")
}

// Exercises the natural cron-tick path. Most dpol tests pre-seed an old
// LastExecutionTime to force immediate execution (useful so individual tests
// stay fast), but the controller's real scheduling loop (queue.AddAfter based
// on cron.Next()) is never covered by those. This test creates a policy with
// no pre-seed, schedule "* * * * *", and waits up to 70 seconds for the
// controller to fire on its own when the minute boundary passes.
//
// Skipped under "go test -short" because it adds ~60s to the suite.
func TestDeletingPolicy_ExecutesOnNaturalCronTick(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cron-tick test (up to 70s) in short mode")
	}
	ns := "dpol-cron-tick"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "target", ns, map[string]string{"cleanup": "true"})

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-cron-tick"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: configMapMatchRules(),
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "in-namespace",
				Expression: `object.metadata.namespace == "dpol-cron-tick"`,
			}},
		},
	}
	createDpolWithCleanup(t, policy)

	// Wait up to 70 seconds for the controller to pick up the policy, compute
	// the next top-of-minute via cron, and fire when the delay expires. Max
	// possible delay for "* * * * *" is 60s + some scheduling slack.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "target", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 70*time.Second, 1*time.Second, "target should be deleted after the controller fires on the natural cron tick")

	// Confirm the controller (not us) wrote LastExecutionTime.
	pol, err := testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies().Get(context.Background(), policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, pol.Status.LastExecutionTime.IsZero(), "controller should have set LastExecutionTime after firing")
}

// Tenant safety: a NamespacedDeletingPolicy authored in team-a's namespace must
// not delete cluster-scoped resources, even if a user accidentally points it
// at one. The controller skips cluster-scoped GVRs for namespaced policies
// (controller.go skips at the `policyNamespace != "" && !isNamespaced(gvr)` guard).
//
// The test creates a NamespacedDeletingPolicy that targets `Namespaces` (cluster-scoped)
// from inside one namespace, then asserts:
//  1. the target namespace is not deleted (and not even marked for deletion).
//  2. LastExecutionTime still advances, proving the controller ran the reconcile loop
//     and skipped the GVR rather than erroring out.
func TestNamespacedDeletingPolicy_SkipsClusterScopedResource(t *testing.T) {
	nsPolicy := "ndpol-cluster-skip"
	nsTarget := "ndpol-cluster-skip-target"
	framework.CreateNamespace(t, testEnv.KubeClient, nsPolicy)
	framework.CreateNamespace(t, testEnv.KubeClient, nsTarget)
	deps.EventCapture.Clear()

	policy := &policiesv1beta1.NamespacedDeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ndpol-target-namespaces",
			Namespace: nsPolicy,
		},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: matchRulesFor("", "v1", "namespaces"),
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "always",
				Expression: "true",
			}},
		},
	}
	createNdpolWithCleanup(t, policy)
	triggerNdpolExecution(t, nsPolicy, policy.Name)

	// Wait for the controller to record execution.
	require.Eventually(t, func() bool {
		pol, err := testEnv.KyvernoClient.PoliciesV1beta1().NamespacedDeletingPolicies(nsPolicy).Get(context.Background(), policy.Name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return !pol.Status.LastExecutionTime.IsZero() &&
			time.Since(pol.Status.LastExecutionTime.Time) < 30*time.Second
	}, 15*time.Second, 200*time.Millisecond, "LastExecutionTime should advance even when the targeted GVR is skipped")

	// Target namespace must not be deleted nor marked for deletion. envtest has no
	// kube-controller-manager / GC, so even a "deleted" namespace would linger as
	// Terminating; checking DeletionTimestamp is the strict assertion.
	target, err := testEnv.KubeClient.CoreV1().Namespaces().Get(context.Background(), nsTarget, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Nil(t, target.DeletionTimestamp, "namespaced policy must not initiate deletion of a cluster-scoped resource")
}

// Platform engineer wants cleanup to apply only in dev namespaces (labeled
// env=dev). The policy uses `namespaceObject.metadata.labels` in its CEL
// condition. Only resources living in labeled namespaces are deleted.
//
// This exercises engine.go's namespace resolution path, which populates the
// `namespaceObject` CEL key by calling nsResolver.Get(namespace) against the
// namespace lister.
func TestDeletingPolicy_NamespaceObjectInConditions(t *testing.T) {
	nsDev := "dpol-nsobj-dev"
	nsProd := "dpol-nsobj-prod"
	framework.CreateNamespace(t, testEnv.KubeClient, nsDev)
	framework.CreateNamespace(t, testEnv.KubeClient, nsProd)
	labelNamespace(t, nsDev, map[string]string{"env": "dev"})
	deps.EventCapture.Clear()

	createConfigMap(t, "in-dev", nsDev, nil)
	createConfigMap(t, "in-prod", nsProd, nil)

	// Cluster-scoped policy. NamespaceSelector restricts the controller's list
	// to just the two test namespaces; without it the policy would scan every
	// namespace in envtest and could collide with parallel test data.
	rules := configMapMatchRules()
	rules.NamespaceSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      "kubernetes.io/metadata.name",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{nsDev, nsProd},
		}},
	}

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-namespaceobject"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: rules,
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "only-dev-namespaces",
				Expression: `has(namespaceObject.metadata.labels) && namespaceObject.metadata.labels["env"] == "dev"`,
			}},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// dev ConfigMap should be deleted.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(nsDev).Get(context.Background(), "in-dev", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "in-dev should be deleted (namespace labeled env=dev)")

	// prod ConfigMap should survive (its namespace lacks the env=dev label).
	cm, err := testEnv.KubeClient.CoreV1().ConfigMaps(nsProd).Get(context.Background(), "in-prod", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "in-prod", cm.Name)
}

// Safety guarantee: a CEL evaluation error during condition evaluation must
// NOT result in a deletion. A user with a typo or a broken expression in their
// policy should never lose data because of it. The controller's policy.go
// returns an error from match() which engine.go propagates, and the deleting
// controller skips the resource (controller.go: append err to errs and continue).
//
// To force a runtime error, a variable references a missing key on a DynType
// (`object.neverExists.attr`). CEL constant-folds simple errors like `1/0` to
// compile time, so this DynType pattern is the reliable way to guarantee a
// runtime evaluation error. The variable is then referenced from a condition
// to actually surface the lazy-evaluation error.
func TestDeletingPolicy_CELEvaluationErrorSkipsDeletion(t *testing.T) {
	ns := "dpol-cel-error"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "should-survive", ns, nil)

	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-cel-eval-error"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:         "* * * * *",
			MatchConstraints: configMapMatchRules(),
			Variables: []admissionregistrationv1.Variable{{
				Name:       "broken",
				Expression: `object.neverExists.attr`,
			}},
			Conditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "in-namespace",
					Expression: `object.metadata.namespace == "dpol-cel-error"`,
				},
				{
					// Reads variables.broken to force lazy evaluation (and the error).
					Name:       "uses-broken-var",
					Expression: `variables.broken == "anything"`,
				},
			},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	// Safety contract: a CEL eval error must never result in deletion. When
	// engine.Handle returns an error, the controller appends to its error
	// slice and continues. After processing all resources, deleting() returns
	// the multierr, reconcile() early-exits, and the worker requeues with
	// backoff. LastExecutionTime is intentionally NOT updated, because each
	// retry hits the same eval error. require.Never holds the assertion across
	// multiple retries to catch a regression where eval errors silently allow
	// deletion.
	require.Never(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "should-survive", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 5*time.Second, 500*time.Millisecond, "ConfigMap must not be deleted when CEL evaluation errors")
}

// Two teams accidentally author overlapping cleanup policies that target the
// same resource. With 3 controller workers, both reconciles can run nearly
// concurrently. Whichever policy fires first deletes the resource; the second
// policy hits the resource gone and goes through the controller's NotFound
// branch (controller.go IsNotFound → continue). That branch must not emit a
// failure event - the resource being already gone is expected, not an error.
func TestDeletingPolicy_MultiplePoliciesSameResource(t *testing.T) {
	ns := "dpol-multi-policies"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "shared-target", ns, map[string]string{"cleanup": "true"})

	makePolicy := func(name string) *policiesv1beta1.DeletingPolicy {
		rules := configMapMatchRules()
		// Pin both policies to this test's namespace to bound blast radius
		// (cluster-scoped DPols list resources cluster-wide otherwise).
		rules.NamespaceSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"kubernetes.io/metadata.name": ns},
		}
		return &policiesv1beta1.DeletingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: policiesv1beta1.DeletingPolicySpec{
				Schedule:         "* * * * *",
				MatchConstraints: rules,
				Conditions: []admissionregistrationv1.MatchCondition{{
					Name:       "has-cleanup-label",
					Expression: `has(object.metadata.labels) && object.metadata.labels["cleanup"] == "true"`,
				}},
			},
		}
	}
	pol1 := makePolicy("dpol-multi-1")
	pol2 := makePolicy("dpol-multi-2")
	createDpolWithCleanup(t, pol1)
	createDpolWithCleanup(t, pol2)

	triggerDpolExecution(t, pol1.Name)
	triggerDpolExecution(t, pol2.Name)

	// At least one policy must have deleted the target.
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "shared-target", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "shared-target should be deleted by one of the overlapping policies")

	// Wait for both policies to finish their reconcile. After this point the
	// "loser" has already taken its NotFound path (or skipped the resource
	// entirely if the listing happened post-delete) - either way, the next
	// assertion is meaningful.
	require.Eventually(t, func() bool {
		p1, err := testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies().Get(context.Background(), pol1.Name, metav1.GetOptions{})
		if err != nil || p1.Status.LastExecutionTime.IsZero() {
			return false
		}
		p2, err := testEnv.KyvernoClient.PoliciesV1beta1().DeletingPolicies().Get(context.Background(), pol2.Name, metav1.GetOptions{})
		if err != nil || p2.Status.LastExecutionTime.IsZero() {
			return false
		}
		return time.Since(p1.Status.LastExecutionTime.Time) < 30*time.Second &&
			time.Since(p2.Status.LastExecutionTime.Time) < 30*time.Second
	}, 15*time.Second, 200*time.Millisecond, "both policies should record an execution")

	// The NotFound branch must not generate failure events.
	events := deps.EventCapture.GetEvents()
	for _, e := range events {
		assert.NotEqual(t, event.PolicyError, e.Reason,
			"PolicyError event from %s/%s on resource %s/%s indicates the NotFound branch is incorrectly emitting failures",
			e.Source, e.Action, e.Regarding.Namespace, e.Regarding.Name)
	}
}

// Smoke-test that DeletionPropagationPolicy on the policy spec is wired through
// to the dclient DeleteOptions when the controller deletes a resource. envtest
// has no kube-controller-manager and therefore no garbage collector, so the
// real cascading semantics (Foreground waits for dependents, Background lets GC
// clean them up async, Orphan abandons them) cannot be verified at the GC level
// here. What we CAN verify is that setting the policy field doesn't cause an
// API error and the targeted object is deleted - which is enough to catch a
// regression in the wiring.
//
// Background is chosen deliberately: Foreground would leave the object stuck
// in Terminating in envtest (no GC to finish it), defeating IsNotFound checks.
func TestDeletingPolicy_DeletionPropagationPolicy(t *testing.T) {
	ns := "dpol-propagation"
	framework.CreateNamespace(t, testEnv.KubeClient, ns)
	deps.EventCapture.Clear()

	createConfigMap(t, "to-delete", ns, nil)

	background := metav1.DeletePropagationBackground
	policy := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "dpol-with-propagation"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule:                  "* * * * *",
			MatchConstraints:          configMapMatchRules(),
			DeletionPropagationPolicy: &background,
			Conditions: []admissionregistrationv1.MatchCondition{{
				Name:       "in-namespace",
				Expression: `object.metadata.namespace == "dpol-propagation"`,
			}},
		},
	}
	createDpolWithCleanup(t, policy)
	triggerDpolExecution(t, policy.Name)

	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().ConfigMaps(ns).Get(context.Background(), "to-delete", metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 15*time.Second, 200*time.Millisecond, "to-delete should be deleted with Background propagation")
}
