package pss

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/policy"
)

// getPodWithMatchingContainers extracts matching container/pod info by the given exclude rule
// and returns pod manifests containing spec and container info respectively
func getPodWithMatchingContainers(exclude kyvernov1.PodSecurityStandard, pod *corev1.Pod) (podSpec, matching *corev1.Pod) {
	if len(exclude.Images) == 0 {
		podSpec = pod.DeepCopy()
		podSpec.Spec.Containers = []corev1.Container{{Name: "fake"}}
		podSpec.Spec.InitContainers = nil
		podSpec.Spec.EphemeralContainers = nil
		return podSpec, nil
	}

	matchingImages := exclude.Images
	matching = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
		},
	}
	for _, container := range pod.Spec.Containers {
		if utils.ContainsWildcardPatterns(matchingImages, container.Image) {
			matching.Spec.Containers = append(matching.Spec.Containers, container)
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if utils.ContainsWildcardPatterns(matchingImages, container.Image) {
			matching.Spec.InitContainers = append(matching.Spec.InitContainers, container)
		}
	}

	for _, container := range pod.Spec.EphemeralContainers {
		if utils.ContainsWildcardPatterns(matchingImages, container.Image) {
			matching.Spec.EphemeralContainers = append(matching.Spec.EphemeralContainers, container)
		}
	}

	return nil, matching
}

// Get restrictedFields from Check.ID
func getRestrictedFields(check policy.Check) []restrictedField {
	for _, control := range PSS_controls_to_check_id {
		for _, checkID := range control {
			if check.ID == checkID {
				return PSS_controls[checkID]
			}
		}
	}
	return nil
}

func FormatChecksPrint(checks []pssCheckResult) string {
	var str string
	for _, check := range checks {
		str += fmt.Sprintf("(%+v)\n", check.checkResult)
	}
	return str
}
