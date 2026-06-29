//go:build integration

package gpol_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/gpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// createSecretWithCleanup creates a Secret and registers cleanup. The returned
// Secret carries a ResourceVersion, which addGenerateLabels (pkg/cel/libs/context.go)
// requires to stamp generate.kyverno.io/source-uid on the downstream so the
// WatchManager can link source events to downstreams.
func createSecretWithCleanup(t *testing.T, name, namespace string, data map[string][]byte) *corev1.Secret {
	t.Helper()
	ctx := context.Background()
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
	}
	created, err := testEnv.KubeClient.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{})
	require.NoError(t, err, "create source Secret %s/%s", namespace, name)
	t.Cleanup(func() {
		bg := metav1.DeletePropagationBackground
		_ = testEnv.KubeClient.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{PropagationPolicy: &bg})
	})
	return created
}

func updateSecretData(t *testing.T, name, namespace string, data map[string][]byte) {
	t.Helper()
	ctx := context.Background()
	sec, err := testEnv.KubeClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err)
	sec.Data = data
	_, err = testEnv.KubeClient.CoreV1().Secrets(namespace).Update(ctx, sec, metav1.UpdateOptions{})
	require.NoError(t, err)
}

// deleteSecret uses Background propagation; envtest has no kube-controller-manager,
// so Foreground would leave the Secret stuck Terminating.
func deleteSecret(t *testing.T, name, namespace string) {
	t.Helper()
	bg := metav1.DeletePropagationBackground
	err := testEnv.KubeClient.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{PropagationPolicy: &bg})
	require.NoError(t, err)
}

func waitForSecretPresent(t *testing.T, name, namespace string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return err == nil
	}, 10*time.Second, 200*time.Millisecond, "Secret %s/%s should exist", namespace, name)
}

func waitForSecretGone(t *testing.T, name, namespace string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := testEnv.KubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		return apierrors.IsNotFound(err)
	}, 10*time.Second, 200*time.Millisecond, "Secret %s/%s should be gone", namespace, name)
}

func waitForSecretData(t *testing.T, name, namespace, key, expected string) {
	t.Helper()
	require.Eventually(t, func() bool {
		sec, err := testEnv.KubeClient.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return string(sec.Data[key]) == expected
	}, 10*time.Second, 200*time.Millisecond, "Secret %s/%s should have data[%q] == %q", namespace, name, key, expected)
}

// buildSyncClonePolicy mirrors the chainsaw clone/sync policy shape: namespace
// trigger, clone from "default", sync enabled. The matchCondition pin to a single
// target namespace is the blast-radius safeguard; without it the cluster-scoped
// gpol fires on every namespace event in envtest and contaminates parallel tests.
func buildSyncClonePolicy(policyName, sourceName, sourceResource, restrictToNamespace string) *policiesv1beta1.GeneratingPolicy {
	return &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
					Enabled: ptr.To(true),
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"namespaces"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "only-test-namespace",
				Expression: fmt.Sprintf("object.metadata.name == %q", restrictToNamespace),
			}},
			Variables: []admissionregistrationv1.Variable{
				{Name: "nsName", Expression: "object.metadata.name"},
				{Name: "source", Expression: fmt.Sprintf(`resource.get("v1", %q, "default", %q)`, sourceResource, sourceName)},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: "generator.apply(variables.nsName, [variables.source])"},
			},
		},
	}
}

func namespaceJSON(name string) []byte {
	return []byte(fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {"name": %q, "uid": "ns-uid-%s"}
	}`, name, name))
}

// setupSyncTest provisions the source Secret, policy, target namespace, and
// sync-aware processor; fires the trigger admission once; and waits for the
// initial downstream clone. Tests focus on their scenario-specific action and
// the assertion that proves sync reacted.
func setupSyncTest(t *testing.T, sourceName, policyName, targetNs string, sourceData map[string][]byte) {
	t.Helper()
	createSecretWithCleanup(t, sourceName, "default", sourceData)

	policy := buildSyncClonePolicy(policyName, sourceName, "secrets", targetNs)
	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, policyName)
	framework.CreateNamespace(t, testEnv.KubeClient, targetNs)

	// WatchManager must be constructed after TestEnv.Start so dclient discovery is ready.
	wm, stopWM := framework.NewGpolWatchManager(testEnv.DClient, logr.Discard())
	t.Cleanup(stopWM)
	processor := framework.NewURProcessorWithSyncWatchers(gpolEngine, gpolProvider, testEnv.ContextProvider, wm)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister, "")

	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.Generate(ctx, logr.Discard(), framework.NamespaceAdmissionRequest(targetNs, namespaceJSON(targetNs)), "", time.Now())
	require.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 10*time.Second, 200*time.Millisecond, "UR not processed in time")
	require.Empty(t, mock.ProcessingErrors())

	waitForSecretPresent(t, sourceName, targetNs)
}

// --- Tests ---

// TestGenerateSync_SourceModification_PropagatesToDownstream ports chainsaw
// scenario sync-modify-source: when the source Secret is mutated, every cloned
// downstream picks up the change. Exercises dynamic_watcher.handleUpdate, source
// branch (uid not in cache → list by generate.kyverno.io/source-uid → update each).
func TestGenerateSync_SourceModification_PropagatesToDownstream(t *testing.T) {
	const (
		sourceName    = "sync-mod-src"
		targetNs      = "sync-mod-src-target"
		policyName    = "gen-sync-mod-src"
		initialValue  = "initial-secret-value"
		modifiedValue = "rotated-secret-value"
	)

	setupSyncTest(t, sourceName, policyName, targetNs, map[string][]byte{
		"foo": []byte(initialValue),
	})

	waitForSecretData(t, sourceName, targetNs, "foo", initialValue)
	initial, err := testEnv.KubeClient.CoreV1().Secrets(targetNs).Get(context.Background(), sourceName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "kyverno", initial.Labels["app.kubernetes.io/managed-by"], "downstream should carry the kyverno managed-by label")

	updateSecretData(t, sourceName, "default", map[string][]byte{
		"foo": []byte(modifiedValue),
	})
	waitForSecretData(t, sourceName, targetNs, "foo", modifiedValue)
}

// TestGenerateSync_SourceDeletion_RemovesDownstream ports chainsaw scenario
// sync-delete-source: when the source is deleted, every clone disappears.
// Exercises dynamic_watcher.handleDelete, source branch (no kyverno managed-by
// label → list downstreams by source-uid → delete each).
func TestGenerateSync_SourceDeletion_RemovesDownstream(t *testing.T) {
	const (
		sourceName = "sync-del-src"
		targetNs   = "sync-del-src-target"
		policyName = "gen-sync-del-src"
	)

	setupSyncTest(t, sourceName, policyName, targetNs, map[string][]byte{
		"foo": []byte("retiring-template"),
	})

	deleteSecret(t, sourceName, "default")
	waitForSecretGone(t, sourceName, targetNs)
}

// TestGenerateSync_DownstreamDrift_RevertsToSource ports chainsaw scenario
// sync-modify-downstream: when a user tampers with the generated copy, sync
// reverts it. Exercises dynamic_watcher.handleUpdate, downstream branch (uid in
// cache, hash differs from cached hash → UpdateResource with cached content).
func TestGenerateSync_DownstreamDrift_RevertsToSource(t *testing.T) {
	const (
		sourceName   = "sync-drift-src"
		targetNs     = "sync-drift-target"
		policyName   = "gen-sync-drift"
		sourceValue  = "source-of-truth"
		driftedValue = "user-tampered-value"
	)

	setupSyncTest(t, sourceName, policyName, targetNs, map[string][]byte{
		"foo": []byte(sourceValue),
	})

	waitForSecretData(t, sourceName, targetNs, "foo", sourceValue)
	updateSecretData(t, sourceName, targetNs, map[string][]byte{
		"foo": []byte(driftedValue),
	})
	waitForSecretData(t, sourceName, targetNs, "foo", sourceValue)
}

// TestGenerateSync_DownstreamDeletion_Recreates ports chainsaw scenario
// sync-delete-downstream: when a user deletes the generated copy, sync recreates
// it. Exercises dynamic_watcher.handleDelete, downstream branch (kyverno-managed
// label set, uid in cache → CreateResource from cached content).
func TestGenerateSync_DownstreamDeletion_Recreates(t *testing.T) {
	const (
		sourceName  = "sync-rec-src"
		targetNs    = "sync-rec-target"
		policyName  = "gen-sync-recreate"
		sourceValue = "must-survive-deletion"
	)

	setupSyncTest(t, sourceName, policyName, targetNs, map[string][]byte{
		"foo": []byte(sourceValue),
	})

	waitForSecretData(t, sourceName, targetNs, "foo", sourceValue)
	// waitForSecretData (not waitForSecretPresent) so the post-recreate check
	// also proves the recreated Secret carries the source content.
	deleteSecret(t, sourceName, targetNs)
	waitForSecretData(t, sourceName, targetNs, "foo", sourceValue)
}
