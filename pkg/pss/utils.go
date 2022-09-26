package pss

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/pod-security-admission/policy"
)

// getPodWithMatchingContainers extracts matching container/pod info by the given exclude rule
// and returns pod manifests containing spec and container info respectively
func getPodWithMatchingContainers(exclude kyvernov1.PodSecurityStandard, pod *corev1.Pod) (podSpec, matching *corev1.Pod) {
	if len(exclude.Images) == 0 {
		*podSpec = *pod
		podSpec.Spec.Containers = []corev1.Container{{Name: "fake"}}
		podSpec.Spec.InitContainers = []corev1.Container{}
		podSpec.Spec.EphemeralContainers = []corev1.EphemeralContainer{}
		return podSpec, nil
	}

	matchingImages := exclude.Images
	for _, container := range pod.Spec.Containers {
		if utils.ContainsWildcardPatterns(matchingImages, container.Image) {
			matching.Spec.Containers = append(pod.Spec.Containers, container)
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if utils.ContainsWildcardPatterns(matchingImages, container.Image) {
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
		}
	}

	for _, container := range pod.Spec.EphemeralContainers {
		if utils.ContainsWildcardPatterns(matchingImages, container.Image) {
			pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, container)
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
