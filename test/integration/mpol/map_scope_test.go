//go:build integration

package mpol_test

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
)

// TestGeneratedMutatingAdmissionPolicyPreservesScope verifies end-to-end against kube-apiserver
// that a native v1 MutatingAdmissionPolicy built from a scoped MutatingPolicy enforces
// NamespaceSelector, ObjectSelector, ExcludeResourceRules, and MatchPolicy rather than mutating
// resources across every namespace cluster-wide.
func TestGeneratedMutatingAdmissionPolicyPreservesScope(t *testing.T) {
	ctx := context.Background()

	suffix := rand.String(5)
	targetNS := "map-scope-target-" + suffix
	otherNS := "map-scope-other-" + suffix
	policyName := "scope-bounded-label-" + suffix
	mapName := "mpol-" + policyName
	bindingName := mapName + "-binding"

	// Create target namespace (matching NamespaceSelector) and an unrelated namespace
	// (representing system/unselected namespaces that must never be mutated).
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, targetNS, map[string]string{"map-scope": "enabled"})
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, otherNS, map[string]string{"map-scope": "disabled"})

	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
			UID:  types.UID("uid-" + policyName),
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"map-scope": "enabled"},
				},
				ObjectSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "skip-mutation",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					}},
				},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"autoscaling"},
								APIVersions: []string{"v1"},
								Resources:   []string{"horizontalpodautoscalers"},
							},
						},
					},
				},
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					ResourceNames: []string{"excluded-pod"},
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
				MatchPolicy: ptr.To(admissionregistrationv1.Exact),
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{metadata: Object.metadata{labels: {"mutated-by-map": "true"}}}`,
				},
			}},
		},
	}

	mapol := &admissionregistrationv1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: mapName},
	}
	admissionpolicy.BuildMutatingAdmissionPolicyV1(mapol, mp, nil)
	// Clear OwnerReferences since no persisted MutatingPolicy with this fake UID exists in apiserver.
	mapol.OwnerReferences = nil

	binding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
	}
	admissionpolicy.BuildMutatingAdmissionPolicyBindingV1(binding, mp)
	binding.OwnerReferences = nil

	_, err := testEnv.KubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Create(ctx, mapol, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = testEnv.KubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Delete(context.Background(), mapName, metav1.DeleteOptions{})
	})

	_, err = testEnv.KubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = testEnv.KubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Delete(context.Background(), bindingName, metav1.DeleteOptions{})
	})

	newPod := func(name string, labels map[string]string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}},
			},
		}
	}

	// Wait until kube-apiserver's MAP informer has activated the newly created policy and binding.
	require.Eventually(t, func() bool {
		pod, err := testEnv.KubeClient.CoreV1().Pods(targetNS).Create(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "probe-"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}}},
		}, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		return err == nil && pod.Labels["mutated-by-map"] == "true"
	}, 10*time.Second, 100*time.Millisecond, "native MutatingAdmissionPolicy did not become active in kube-apiserver")

	podCases := []struct {
		name        string
		namespace   string
		podName     string
		labels      map[string]string
		wantMutated bool
		reason      string
	}{
		{
			name:        "matching pod in target namespace is mutated",
			namespace:   targetNS,
			podName:     "allowed-pod",
			wantMutated: true,
		},
		{
			name:      "pod in non-matching namespace is not mutated",
			namespace: otherNS,
			podName:   "other-ns-pod",
			reason:    "NamespaceSelector must prevent mutating pods outside matching namespaces",
		},
		{
			name:      "pod failing ObjectSelector is not mutated",
			namespace: targetNS,
			podName:   "opted-out-pod",
			labels:    map[string]string{"skip-mutation": "true"},
			reason:    "ObjectSelector must prevent mutating opted-out pods",
		},
		{
			name:      "pod matching ExcludeResourceRules is not mutated",
			namespace: targetNS,
			podName:   "excluded-pod",
			reason:    "ExcludeResourceRules must prevent mutating excluded resource names",
		},
	}

	for _, tc := range podCases {
		t.Run(tc.name, func(t *testing.T) {
			pod, err := testEnv.KubeClient.CoreV1().Pods(tc.namespace).Create(ctx, newPod(tc.podName, tc.labels), metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
			require.NoError(t, err)
			if tc.wantMutated {
				assert.Equal(t, "true", pod.Labels["mutated-by-map"])
			} else {
				assert.NotContains(t, pod.Labels, "mutated-by-map", tc.reason)
			}
		})
	}

	t.Run("MatchPolicy Exact matches v1 HPA and ignores v2 HPA", func(t *testing.T) {
		hpaV1, err := testEnv.KubeClient.AutoscalingV1().HorizontalPodAutoscalers(targetNS).Create(ctx, &autoscalingv1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: "hpa-v1"},
			Spec: autoscalingv1.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Name: "app"},
				MaxReplicas:    2,
			},
		}, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		require.NoError(t, err)
		assert.Equal(t, "true", hpaV1.Labels["mutated-by-map"])

		hpaV2, err := testEnv.KubeClient.AutoscalingV2().HorizontalPodAutoscalers(targetNS).Create(ctx, &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: "hpa-v2"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Deployment", Name: "app"},
				MaxReplicas:    2,
			},
		}, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		require.NoError(t, err)
		assert.NotContains(t, hpaV2.Labels, "mutated-by-map", "MatchPolicy Exact must not mutate autoscaling/v2 when rule specifies autoscaling/v1")
	})
}
