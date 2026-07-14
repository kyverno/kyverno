//go:build integration

package dpol_test

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
