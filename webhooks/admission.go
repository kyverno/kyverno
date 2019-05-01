package webhooks

import (
	"fmt"
	"regexp"

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
func IsRuleApplicableToRequest(policyResource types.PolicyResource, request *v1beta1.AdmissionRequest) (bool, error) {
	return IsRuleApplicableToResource(request.Kind.Kind, request.Object.Raw, policyResource)
}

// kind is the type of object being manipulated
// Checks requests kind, name and labels to fit the policy
func IsRuleApplicableToResource(kind string, resourceRaw []byte, policyResource types.PolicyResource) (bool, error) {
	if policyResource.Kind != kind {
		return false, nil
	}

	if resourceRaw != nil {
		meta := parseMetadataFromObject(resourceRaw)
		name := parseNameFromMetadata(meta)

		// if policyResource.Name != nil && *policyResource.Name != name {
		// 	return false, false
		// }
		if policyResource.Name != nil {
			fmt.Println("*policyResource.Name, name", *policyResource.Name, name)

			// if no regex used, check if names are matched, return directly
			if policyResource.Name != nil && *policyResource.Name == name {
				return true, nil
			}

			// validation of regex is peformed when validating the policyResource
			// refer to policyResource.Validate()
			parseRegexPolicyResourceName(*policyResource.Name)
			match, _ := regexp.MatchString(*policyResource.Name, name)

			if !match {
				return false, nil
			}

			if policyResource.Selector != nil {
				selector, err := metav1.LabelSelectorAsSelector(policyResource.Selector)

				if err != nil {
					return false, err
				}

				labelMap := parseLabelsFromMetadata(meta)

				if !selector.Matches(labelMap) {
					return false, nil
				}
			}
		}
	}
	return true, nil
}
