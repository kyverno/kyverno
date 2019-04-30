package webhooks

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var supportedKinds = [...]string{
	"ConfigMap",
	"CronJob",
	"DaemonSet",
	"Deployment",
	"Endpoints",
	"HorizontalPodAutoscaler",
	"Ingress",
	"Job",
	"LimitRange",
	"Namespace",
	"NetworkPolicy",
	"PersistentVolumeClaim",
	"PodDisruptionBudget",
	"PodTemplate",
	"ResourceQuota",
	"Secret",
	"Service",
	"StatefulSet",
}

func kindIsSupported(kind string) bool {
	for _, k := range supportedKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Checks for admission if kind is supported
func AdmissionIsRequired(request *v1beta1.AdmissionRequest) bool {
	// Here you can make additional hardcoded checks
	return kindIsSupported(request.Kind.Kind)
}

// Checks requests kind, name and labels to fit the policy
func IsRuleApplicableToRequest(policyResource types.PolicyResource, request *v1beta1.AdmissionRequest) bool {
	return IsRuleApplicableToResource(request.Kind.Kind, request.Object.Raw, policyResource)
	// if policyResource.Kind != request.Kind.Kind {
	// 	return false
	// }

	// if request.Object.Raw != nil {
	// 	meta := parseMetadataFromObject(request.Object.Raw)
	// 	name := parseNameFromMetadata(meta)

	// 	if policyResource.Name != nil && *policyResource.Name != name {
	// 		return false
	// 	}

	// 	if policyResource.Selector != nil {
	// 		selector, err := metav1.LabelSelectorAsSelector(policyResource.Selector)

	// 		if err != nil {
	// 			return false
	// 		}

	// 		labelMap := parseLabelsFromMetadata(meta)

	// 		if !selector.Matches(labelMap) {
	// 			return false
	// 		}
	// 	}
	// }
	// return true
}

// kind is the type of object being manipulated
func IsRuleApplicableToResource(kind string, resourceRaw []byte, policyResource types.PolicyResource) bool {
	if policyResource.Kind != kind {
		return false
	}

	if resourceRaw != nil {
		meta := parseMetadataFromObject(resourceRaw)
		name := parseNameFromMetadata(meta)

		if policyResource.Name != nil && *policyResource.Name != name {
			return false
		}

		if policyResource.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policyResource.Selector)

			if err != nil {
				return false
			}

			labelMap := parseLabelsFromMetadata(meta)

			if !selector.Matches(labelMap) {
				return false
			}
		}
	}

	return true
}
