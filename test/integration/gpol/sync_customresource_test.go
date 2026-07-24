//go:build integration

package gpol_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/gpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

// queueGVR is the cluster scoped custom resource these tests generate. It mirrors the Kai Scheduler
// Queue an end user reported syncing alongside a ResourceQuota and a NetworkPolicy.
var queueGVR = schema.GroupVersionResource{Group: "scheduling.run.ai", Version: "v2", Resource: "queues"}

// requireQueueCRDServed confirms the Queue CRD that TestMain installs is being served before a test
// relies on it.
func requireQueueCRDServed(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := testEnv.DClient.GetDynamicInterface().Resource(queueGVR).List(context.Background(), metav1.ListOptions{})
		return err == nil
	}, 20*time.Second, 250*time.Millisecond, "Queue CRD is not served in time")
}

// createTierConfigMap seeds the ConfigMap the policy reads with resource.Get, so the generated
// resources are derived from cluster data the way a real tiering policy does it. Both the namespace
// and the ConfigMap are shared fixtures created on first use and left in place: envtest runs without
// a namespace controller, so a deleted namespace stays Terminating and cannot be recreated by the
// next test.
func createTierConfigMap(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "naas-system",
		Labels: map[string]string{"kubernetes.io/metadata.name": "naas-system"},
	}}
	if _, err := testEnv.KubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		require.True(t, apierrors.IsAlreadyExists(err), "create naas-system namespace: %v", err)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "naas-tier-config", Namespace: "naas-system"},
		Data: map[string]string{
			"free": "cpu:\n  request: \"1\"\n  limit: \"2\"\nmemory:\n  request: 1Gi\n  limit: 2Gi\nkai:\n  cpu: \"1\"\n  memory: 1Gi\n",
		},
	}
	if _, err := testEnv.KubeClient.CoreV1().ConfigMaps("naas-system").Create(ctx, cm, metav1.CreateOptions{}); err != nil {
		require.True(t, apierrors.IsAlreadyExists(err), "create tier ConfigMap: %v", err)
	}
}

// buildTenantPolicy mirrors the user reported policy: for a tenant namespace it generates and syncs a
// namespaced ResourceQuota, a cluster scoped Queue custom resource, and a namespaced NetworkPolicy,
// with the quota and the queue derived from a tier ConfigMap. matchConditions pin the policy to the
// single test namespace, otherwise this cluster scoped policy fires for every namespace in envtest.
func buildTenantPolicy(policyName, userNs string) *policiesv1beta1.GeneratingPolicy {
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
				Expression: fmt.Sprintf("object.metadata.name == %q", userNs),
			}},
			Variables: []admissionregistrationv1.Variable{
				{Name: "nsName", Expression: "object.metadata.name"},
				{Name: "tierValues", Expression: `yaml.parse(resource.Get("v1", "configmaps", "naas-system", "naas-tier-config").data["free"])`},
				{Name: "quota", Expression: `[{
					"kind": dyn("ResourceQuota"),
					"apiVersion": dyn("v1"),
					"metadata": dyn({"name": "tier-quota", "namespace": string(variables.nsName)}),
					"spec": dyn({"hard": dyn({
						"requests.cpu": dyn(string(variables.tierValues.cpu.request)),
						"limits.cpu": dyn(string(variables.tierValues.cpu.limit))
					})})
				}]`},
				{Name: "queue", Expression: `[{
					"kind": dyn("Queue"),
					"apiVersion": dyn("scheduling.run.ai/v2"),
					"metadata": dyn({"name": string(variables.nsName)}),
					"spec": dyn({
						"displayName": dyn("Free Tier"),
						"parentQueue": dyn("normal-root"),
						"resources": dyn({"cpu": dyn({"quota": dyn(variables.tierValues.kai.cpu)})})
					})
				}]`},
				{Name: "networkPolicy", Expression: `[{
					"kind": dyn("NetworkPolicy"),
					"apiVersion": dyn("networking.k8s.io/v1"),
					"metadata": dyn({"name": "namespace-isolation", "namespace": string(variables.nsName)}),
					"spec": dyn({"podSelector": dyn({}), "policyTypes": dyn(["Ingress", "Egress"])})
				}]`},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: "generator.Apply(variables.nsName, variables.quota)"},
				{Expression: `generator.Apply("", variables.queue)`},
				{Expression: "generator.Apply(variables.nsName, variables.networkPolicy)"},
			},
		},
	}
}

// setupTenantSync installs the CRD and tier ConfigMap, creates the policy and tenant namespace, wires
// the production WatchManager into the UR processor, and fires the namespace trigger once. It returns
// once all three downstream resources exist, so tests can act on a fully synced tenant.
func setupTenantSync(t *testing.T, policyName, userNs string) {
	t.Helper()
	requireQueueCRDServed(t)
	createTierConfigMap(t)

	createGpolWithCleanup(t, buildTenantPolicy(policyName, userNs))
	waitForGpolInLister(t, policyName)
	framework.CreateNamespace(t, testEnv.KubeClient, userNs)

	// The WatchManager reads discovery when it is built, so it must come after the Queue CRD is served.
	wm, stopWM := framework.NewGpolWatchManager(testEnv.DClient, logr.Discard())
	t.Cleanup(stopWM)
	processor := framework.NewURProcessorWithSyncWatchers(gpolEngine, gpolProvider, testEnv.ContextProvider, wm)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister, "")

	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.Generate(ctx, logr.Discard(), framework.NamespaceAdmissionRequest(userNs, namespaceJSON(userNs)), "", time.Now())
	require.True(t, resp.Allowed)

	require.Eventually(t, func() bool {
		return len(mock.GetSpecs()) >= 1
	}, 10*time.Second, 200*time.Millisecond, "UR not processed in time")
	require.Empty(t, mock.ProcessingErrors())

	// The cluster scoped Queue outlives the namespace, so remove it explicitly.
	t.Cleanup(func() {
		_ = testEnv.DClient.GetDynamicInterface().Resource(queueGVR).Delete(context.Background(), userNs, metav1.DeleteOptions{})
	})

	waitForQueueField(t, userNs, "displayName", "Free Tier")
	waitForQuotaCPURequest(t, userNs, "1")
	waitForNetworkPolicyTypes(t, userNs, 2)
}

func waitForQueueField(t *testing.T, name, field, expected string) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		q, err := testEnv.DClient.GetDynamicInterface().Resource(queueGVR).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false
		}
		got, _, _ := unstructured.NestedString(q.Object, "spec", field)
		return got == expected
	}, 20*time.Second, 250*time.Millisecond, "Queue %q spec.%s should be %q", name, field, expected)
}

func waitForQuotaCPURequest(t *testing.T, ns, expected string) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		q, err := testEnv.KubeClient.CoreV1().ResourceQuotas(ns).Get(context.Background(), "tier-quota", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return q.Spec.Hard.Name("requests.cpu", "").String() == expected
	}, 20*time.Second, 250*time.Millisecond, "ResourceQuota in %q should have requests.cpu %q", ns, expected)
}

func waitForNetworkPolicyTypes(t *testing.T, ns string, count int) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		np, err := testEnv.KubeClient.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), "namespace-isolation", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return len(np.Spec.PolicyTypes) == count
	}, 20*time.Second, 250*time.Millisecond, "NetworkPolicy in %q should have %d policy types", ns, count)
}

// TestGenerateSync_CustomResourceDownstreamDrift_Reverts is the regression test for synchronization of
// custom resource downstreams. A tenant namespace gets a ResourceQuota, a cluster scoped Queue custom
// resource and a NetworkPolicy; a user then tampers with all three. Synchronization has to revert all
// of them, including the custom resource.
//
// The custom resource is what makes this worth its own test: the API server accepts an update that
// carries no resourceVersion for built-in resources, but rejects it for custom resources, so a revert
// that cleared the resourceVersion silently reverted the ResourceQuota and the NetworkPolicy while
// leaving the Queue tampered with.
func TestGenerateSync_CustomResourceDownstreamDrift_Reverts(t *testing.T) {
	const (
		policyName = "generate-tenant-resources-drift"
		userNs     = "tenant-drift"
	)
	setupTenantSync(t, policyName, userNs)

	ctx := context.Background()
	dyn := testEnv.DClient.GetDynamicInterface()

	// Tamper with the cluster scoped custom resource.
	queue, err := dyn.Resource(queueGVR).Get(ctx, userNs, metav1.GetOptions{})
	require.NoError(t, err)
	require.NoError(t, unstructured.SetNestedField(queue.Object, "Tampered Tier", "spec", "displayName"))
	_, err = dyn.Resource(queueGVR).Update(ctx, queue, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Tamper with the two built-in downstreams, which already reverted correctly, to prove the custom
	// resource is handled the same way rather than being special cased.
	quota, err := testEnv.KubeClient.CoreV1().ResourceQuotas(userNs).Get(ctx, "tier-quota", metav1.GetOptions{})
	require.NoError(t, err)
	quota.Spec.Hard[corev1.ResourceRequestsCPU] = resource.MustParse("64")
	_, err = testEnv.KubeClient.CoreV1().ResourceQuotas(userNs).Update(ctx, quota, metav1.UpdateOptions{})
	require.NoError(t, err)

	np, err := testEnv.KubeClient.NetworkingV1().NetworkPolicies(userNs).Get(ctx, "namespace-isolation", metav1.GetOptions{})
	require.NoError(t, err)
	np.Spec.PolicyTypes = np.Spec.PolicyTypes[:1]
	_, err = testEnv.KubeClient.NetworkingV1().NetworkPolicies(userNs).Update(ctx, np, metav1.UpdateOptions{})
	require.NoError(t, err)

	// All three must be restored to what the policy generated.
	waitForQueueField(t, userNs, "displayName", "Free Tier")
	waitForQuotaCPURequest(t, userNs, "1")
	waitForNetworkPolicyTypes(t, userNs, 2)
}

// TestGenerateSync_CustomResourceDownstream_RestoredAfterRestart covers the other half of the tenant
// scenario: when the background controller restarts its watcher cache is empty and is rebuilt from the
// existing downstreams. A fresh WatchManager has to pick up all three, including the cluster scoped
// custom resource, otherwise the tenant silently loses synchronization for it after every restart.
func TestGenerateSync_CustomResourceDownstream_RestoredAfterRestart(t *testing.T) {
	const (
		policyName = "generate-tenant-resources-restart"
		userNs     = "tenant-restart"
	)
	setupTenantSync(t, policyName, userNs)

	// A restart drops the watcher cache. Rebuild it the way the background controller does, by
	// re-running the policy in cache restore mode and feeding the result to a fresh WatchManager.
	wm, stopWM := framework.NewGpolWatchManager(testEnv.DClient, logr.Discard())
	t.Cleanup(stopWM)

	policy, err := gpolProvider.Get(context.Background(), policyName)
	require.NoError(t, err)

	testEnv.ContextProvider.ClearGeneratedResources()
	request := framework.NamespaceAdmissionRequest(userNs, namespaceJSON(userNs))
	resp, err := gpolEngine.Handle(celengine.RequestFromAdmission(testEnv.ContextProvider, request.AdmissionRequest), policy, true)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.NotNil(t, resp.Policies[0].Result, "cache restore should produce a rule result")

	restored := resp.Policies[0].Result.GeneratedResources()
	require.NoError(t, wm.SyncWatchers(policyName, restored))

	kinds := map[string]bool{}
	for _, d := range wm.GetDownstreams(policyName) {
		kinds[d.GetKind()] = true
	}
	assert.True(t, kinds["ResourceQuota"], "ResourceQuota should be watched again after a restart")
	assert.True(t, kinds["NetworkPolicy"], "NetworkPolicy should be watched again after a restart")
	assert.True(t, kinds["Queue"], "the cluster scoped custom resource should be watched again after a restart")
}
