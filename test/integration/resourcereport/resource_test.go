//go:build integration

package resourcereport_test

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/utils/ptr"
)

// matchResourcesFor selects the given core/v1 resource (e.g. "pods") on CREATE.
func matchResourcesFor(resource string) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{resource},
				},
			},
		}},
	}
}

// TestResourceController_DoesNotWatchBackgroundDisabledPolicyKinds drives the
// real resource-report controller against a real (envtest) API server. It
// creates a background-enabled ValidatingPolicy on Pods and a background-disabled
// one on ConfigMaps, then asserts (via the controller's own metadata cache) that
// a created Pod is tracked (its kind is watched + hashed) while a created
// ConfigMap is not (the disabled policy's kind is never watched). This is the
// end-to-end proof that the fix actually stops the wasted watch/list/hash work
// for background-disabled policies, not just at the kind-set level.
func TestResourceController_DoesNotWatchBackgroundDisabledPolicyKinds(t *testing.T) {
	testEnv, err := framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io", // ValidatingPolicy etc.
		"../../../config/crds/kyverno",             // ClusterPolicy/Policy (the controller always watches these)
	)
	require.NoError(t, err)
	defer testEnv.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	enabled := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "enabled-on-pods"},
		Spec:       policiesv1beta1.ValidatingPolicySpec{MatchConstraints: matchResourcesFor("pods")},
	}
	disabled := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-on-configmaps"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: matchResourcesFor("configmaps"),
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Background: &policiesv1beta1.BackgroundConfiguration{Enabled: ptr.To(false)},
			},
		},
	}
	require.NoError(t, testEnv.Client.Create(ctx, enabled))
	require.NoError(t, testEnv.Client.Create(ctx, disabled))

	// Wire the real controller exactly like cmd/reports-controller/main.go, only
	// the vpol/cpol/pol sources are populated (the rest are not needed here).
	metaClient, err := metadata.NewForConfig(testEnv.Env.Config)
	require.NoError(t, err)
	factory := kyvernoinformer.NewSharedInformerFactory(testEnv.KyvernoClient, 0)
	ctrl := resource.NewController(
		testEnv.DClient,
		factory.Kyverno().V1().Policies(),
		factory.Kyverno().V1().ClusterPolicies(),
		factory.Policies().V1beta1().ValidatingPolicies(),
		nil, nil, nil, nil, nil, // nvpol, mpol, nmpol, ivpol, nivpol
		nil, nil, nil, // vap, map, mapAlpha
		metaClient,
	)
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())

	// Warmup opens the dynamic watchers for the current policy set; Run keeps it
	// reconciling and stops the watchers cleanly when ctx is cancelled.
	require.NoError(t, ctrl.Warmup(ctx))
	go ctrl.Run(ctx, 1)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "probe-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
	}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "probe-cm", Namespace: "default"}}
	require.NoError(t, testEnv.Client.Create(ctx, pod))
	require.NoError(t, testEnv.Client.Create(ctx, cm))

	// The enabled policy's kind (Pod) is watched, so the created Pod lands in the
	// controller's metadata cache.
	require.Eventually(t, func() bool {
		_, _, _, ok := ctrl.GetResourceHash(pod.GetUID())
		return ok
	}, 15*time.Second, 200*time.Millisecond, "the background-enabled policy's kind (Pod) must be watched and tracked")

	// The background-disabled policy's kind (ConfigMap) must never be watched, so
	// the created ConfigMap is not in the cache. Give the watch path the same
	// window the Pod got, to make a false-negative meaningful.
	require.Never(t, func() bool {
		_, _, _, ok := ctrl.GetResourceHash(cm.GetUID())
		return ok
	}, 3*time.Second, 200*time.Millisecond, "the background-disabled policy's kind (ConfigMap) must not be watched")

	// Belt and suspenders: assert the final state explicitly.
	_, _, _, ok := ctrl.GetResourceHash(cm.GetUID())
	assert.False(t, ok, "ConfigMap must not be tracked by the resource controller")
}
