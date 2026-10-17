//go:build integration

package mpol_test

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These tests run the background controller's real mpol processor over envtest (see
// framework.NewMpolBackgroundController). Every name comes from framework.UniqueName, so each test
// is safe to repeat; the race tests are meant to be run with -count=20.

const backgroundTimeout = 30 * time.Second

// backgroundPolicy is an admission-disabled, mutateExisting NamespacedMutatingPolicy on the
// Deployments of one namespace, the shape whose only trigger is a background scan.
func backgroundPolicy(name, namespace, patch string, conditions ...admissionregistrationv1.MatchCondition) *policiesv1beta1.NamespacedMutatingPolicy {
	return &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				Admission:                   &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(false)},
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: ptr.To(true)},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				}},
			},
			MatchConditions: conditions,
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: patch},
			}},
		},
	}
}

func setImagePatch(image string) string {
	return `[JSONPatch{op: "replace", path: "/spec/template/spec/containers/0/image", value: "` + image + `"}]`
}

func addLabelPatch(key string) string {
	return `[JSONPatch{op: "add", path: "/metadata/labels/` + key + `", value: "true"}]`
}

func createDeployment(t *testing.T, namespace, name string, labels map[string]string) {
	t.Helper()
	podLabels := map[string]string{"app": name}
	deployLabels := map[string]string{"app": name}
	for k, v := range labels {
		deployLabels[k] = v
	}
	_, err := testEnv.KubeClient.AppsV1().Deployments(namespace).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: deployLabels},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.26"}}},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func getDeployment(t *testing.T, namespace, name string) *appsv1.Deployment {
	t.Helper()
	deploy, err := testEnv.KubeClient.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return deploy
}

func waitForImage(t *testing.T, namespace, name, image string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return getDeployment(t, namespace, name).Spec.Template.Spec.Containers[0].Image == image
	}, backgroundTimeout, 100*time.Millisecond, "deployment %s/%s never got image %s", namespace, name, image)
}

func waitForURsDone(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		return framework.PendingURCount(t, testEnv) == 0
	}, backgroundTimeout, 100*time.Millisecond, "update requests were not processed in time")
}

// setProviderDelay makes the engine lag behind the policy controller for the rest of the test.
func setProviderDelay(t *testing.T, delay time.Duration) {
	t.Helper()
	testEnv.Mpol.Delayed.SetDelay(delay)
	t.Cleanup(func() { testEnv.Mpol.Delayed.SetDelay(0) })
}

// G15a: background mutateExisting must honour matchConditions (regression #16614, user report
// #17411): only the existing target the condition selects is mutated.
func TestBackground_MutateExisting_HonoursMatchConditions(t *testing.T) {
	framework.NewMpolBackgroundController(t, testEnv, &framework.MockEventGen{})
	ns := framework.CreateUniqueNamespace(t, testEnv.KubeClient, "mpol-bg-conditions")
	createDeployment(t, ns, "web", map[string]string{"tier": "web"})
	createDeployment(t, ns, "db", map[string]string{"tier": "db"})

	policy := backgroundPolicy(framework.UniqueName(t, "web-only"), ns, setImagePatch("nginx:1.27"),
		admissionregistrationv1.MatchCondition{Name: "web-tier", Expression: `object.metadata.labels["tier"] == "web"`})
	createNamespacedPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)
	framework.SubmitCELMutateUR(t, testEnv, ns+"/"+policy.GetName())

	waitForImage(t, ns, "web", "nginx:1.27")
	waitForURsDone(t)
	assert.Equal(t, "nginx:1.26", getDeployment(t, ns, "db").Spec.Template.Spec.Containers[0].Image,
		"a target excluded by matchConditions must not be mutated")
}

// G15b: right after a policy update, the update request must not be processed with the previous
// compiled policy. The provider delay holds the engine on the old version, so without the
// generation check the request applies the old image, completes, and the new one never lands.
func TestBackground_MutateExisting_PolicyUpdateAppliesNewSpec(t *testing.T) {
	framework.NewMpolBackgroundController(t, testEnv, &framework.MockEventGen{})
	ns := framework.CreateUniqueNamespace(t, testEnv.KubeClient, "mpol-bg-update")
	createDeployment(t, ns, "app", nil)

	policy := backgroundPolicy(framework.UniqueName(t, "image-tag"), ns, setImagePatch("nginx:1.27"))
	createNamespacedPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)
	key := ns + "/" + policy.GetName()
	framework.SubmitCELMutateUR(t, testEnv, key)
	waitForImage(t, ns, "app", "nginx:1.27")
	waitForURsDone(t)

	setProviderDelay(t, 2*time.Second)
	updated := &policiesv1beta1.NamespacedMutatingPolicy{}
	require.NoError(t, testEnv.Client.Get(context.Background(), client.ObjectKeyFromObject(policy), updated))
	updated.Spec.Mutations[0].JSONPatch.Expression = setImagePatch("nginx:1.28")
	require.NoError(t, testEnv.Client.Update(context.Background(), updated))
	// what the policy controller does on the update event
	framework.SubmitCELMutateUR(t, testEnv, key)

	waitForImage(t, ns, "app", "nginx:1.28")
	waitForURsDone(t)
}

// G23: two mutateExisting policies writing the same target concurrently must not overwrite each
// other: the processor re-evaluates against the live object when it changed since it was listed.
func TestBackground_MutateExisting_ConcurrentPoliciesKeepBothMutations(t *testing.T) {
	framework.NewMpolBackgroundController(t, testEnv, &framework.MockEventGen{})
	ns := framework.CreateUniqueNamespace(t, testEnv.KubeClient, "mpol-bg-concurrent")
	createDeployment(t, ns, "app", nil)

	first := backgroundPolicy(framework.UniqueName(t, "label-a"), ns, addLabelPatch("a"))
	second := backgroundPolicy(framework.UniqueName(t, "label-b"), ns, addLabelPatch("b"))
	createNamespacedPolicyWithCleanup(t, first)
	createNamespacedPolicyWithCleanup(t, second)
	waitForPolicyReady(t, 2)
	framework.SubmitCELMutateUR(t, testEnv, ns+"/"+first.GetName())
	framework.SubmitCELMutateUR(t, testEnv, ns+"/"+second.GetName())

	waitForURsDone(t)
	require.Eventually(t, func() bool {
		labels := getDeployment(t, ns, "app").GetLabels()
		return labels["a"] == "true" && labels["b"] == "true"
	}, backgroundTimeout, 100*time.Millisecond, "one policy's mutation was lost")
}

// G24: the scan for a new policy can run before the engine has compiled it. It must be retried
// rather than match nothing and complete, which left existing targets unmutated.
func TestBackground_MutateExisting_ScanBeforeCompile(t *testing.T) {
	framework.NewMpolBackgroundController(t, testEnv, &framework.MockEventGen{})
	ns := framework.CreateUniqueNamespace(t, testEnv.KubeClient, "mpol-bg-lag")
	createDeployment(t, ns, "app", nil)
	setProviderDelay(t, 2*time.Second)

	policy := backgroundPolicy(framework.UniqueName(t, "image-tag"), ns, setImagePatch("nginx:1.27"))
	createNamespacedPolicyWithCleanup(t, policy)
	// no waitForPolicyReady: the request races the compile, as on a policy create event
	framework.SubmitCELMutateUR(t, testEnv, ns+"/"+policy.GetName())

	waitForImage(t, ns, "app", "nginx:1.27")
	waitForURsDone(t)
}

// G18 (2): the single background scan of a new namespaced policy, processed right after the
// controller starts on a cold discovery cache, must still find and mutate its target.
func TestBackground_MutateExisting_ColdDiscoveryCache(t *testing.T) {
	ns := framework.CreateUniqueNamespace(t, testEnv.KubeClient, "mpol-bg-cold")
	createDeployment(t, ns, "app", nil)

	framework.NewMpolBackgroundController(t, testEnv, &framework.MockEventGen{})
	policy := backgroundPolicy(framework.UniqueName(t, "image-tag"), ns, setImagePatch("nginx:1.27"))
	createNamespacedPolicyWithCleanup(t, policy)
	framework.SubmitCELMutateUR(t, testEnv, ns+"/"+policy.GetName())

	waitForImage(t, ns, "app", "nginx:1.27")
	waitForURsDone(t)
}
