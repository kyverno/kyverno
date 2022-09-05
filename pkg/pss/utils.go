package pss

import (
	"strings"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/pod-security-admission/policy"
)

func containsContainer(containers interface{}, containerName string) bool {
	switch v := containers.(type) {
	case []interface{}:
		for _, container := range v {
			switch v := container.(type) {
			case corev1.Container:
				if v.Name == containerName {
					return true
				}
			case corev1.EphemeralContainer:
				if v.Name == containerName {
					return true
				}
			}
		}
	case []corev1.Container:
		for _, container := range v {
			if container.Name == containerName {
				return true
			}
		}
	case []corev1.EphemeralContainer:
		for _, container := range v {
			if container.Name == containerName {
				return true
			}
		}
	}
	return false
}

// Get copy of pod with containers (containers, initContainers, ephemeralContainers) matching the exclude.image
func getPodWithMatchingContainers(exclude []kyvernov2beta1.PodSecurityStandard, pod *corev1.Pod) (podCopy corev1.Pod) {
	podCopy = *pod
	podCopy.Spec.Containers = []corev1.Container{}
	podCopy.Spec.InitContainers = []corev1.Container{}
	podCopy.Spec.EphemeralContainers = []corev1.EphemeralContainer{}

	for _, container := range pod.Spec.Containers {
		for _, excludeRule := range exclude {
			// Ignore all restrictedFields when we only specify the `controlName` with no `restrictedField`
			controlNameOnly := excludeRule.RestrictedField == ""
			if !utils.ContainsString(excludeRule.Images, container.Image) {
				continue
			}
			if strings.Contains(excludeRule.RestrictedField, "spec.containers[*]") || controlNameOnly {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.Containers) == 0 || !containsContainer(podCopy.Spec.Containers, container.Name) {
					podCopy.Spec.Containers = append(podCopy.Spec.Containers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.InitContainers {
		for _, excludeRule := range exclude {
			// Ignore all restrictedFields when we only specify the `controlName` with no `restrictedField`
			controlNameOnly := excludeRule.RestrictedField == ""
			if !utils.ContainsString(excludeRule.Images, container.Image) {
				continue
			}
			if strings.Contains(excludeRule.RestrictedField, "spec.initContainers[*]") || controlNameOnly {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.InitContainers) == 0 || !containsContainer(podCopy.Spec.InitContainers, container.Name) {
					podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		for _, excludeRule := range exclude {
			// Ignore all restrictedFields when we only specify the `controlName` with no `restrictedField`
			controlNameOnly := excludeRule.RestrictedField == ""
			if !utils.ContainsString(excludeRule.Images, container.Image) {
				continue
			}
			if strings.Contains(excludeRule.RestrictedField, "spec.ephemeralContainers[*]") || controlNameOnly {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.EphemeralContainers) == 0 || !containsContainer(podCopy.Spec.EphemeralContainers, container.Name) {
					podCopy.Spec.EphemeralContainers = append(podCopy.Spec.EphemeralContainers, container)
				}
			}
		}
	}
	return podCopy
}

// Get containers NOT matching images specified in Exclude values
func getPodWithNotMatchingContainers(exclude []kyvernov2beta1.PodSecurityStandard, pod *corev1.Pod, podWithMatchingContainers *corev1.Pod) (podCopy corev1.Pod) {
	// Only copy containers because we have already evaluated the pod-level controls
	// e.g.: spec.securityContext.hostProcess
	podCopy.Spec.Containers = []corev1.Container{}
	podCopy.Spec.InitContainers = []corev1.Container{}
	podCopy.Spec.EphemeralContainers = []corev1.EphemeralContainer{}

	// Append containers that are not in podWithMatchingContainers already evaluated in EvaluatePod()
	for _, container := range pod.Spec.Containers {
		if !containsContainer(podWithMatchingContainers.Spec.Containers, container.Name) {
			podCopy.Spec.Containers = append(podCopy.Spec.Containers, container)
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if !containsContainer(podWithMatchingContainers.Spec.InitContainers, container.Name) {
			podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, container)
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		if !containsContainer(podWithMatchingContainers.Spec.EphemeralContainers, container.Name) {
			podCopy.Spec.EphemeralContainers = append(podCopy.Spec.EphemeralContainers, container)
		}
	}
	return podCopy
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

func containsContainerLevelControl(restrictedFields []restrictedField) bool {
	for _, restrictedField := range restrictedFields {
		if strings.Contains(restrictedField.path, "ontainers[*]") {
			return true
		}
	}
	return false
}
