package webhooks

import (
	"encoding/json"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var supportedKinds = [...]string{
	"ConfigMap",
	"CronJob",
	"DaemonSet",
	"Deployment",
	"Endpoint",
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

// AdmissionIsRequired checks for admission if kind is supported
func AdmissionIsRequired(request *v1beta1.AdmissionRequest) bool {
	// Here you can make additional hardcoded checks
	return kindIsSupported(request.Kind.Kind)
}

// IsRuleApplicableToRequest checks requests kind, name and labels to fit the policy
func IsRuleApplicableToRequest(policyResource types.PolicyResource, request *v1beta1.AdmissionRequest) bool {
	if policyResource.Selector == nil && policyResource.Name == nil {
		// TBD: selector or name MUST be specified
		return false
	}

	if policyResource.Kind != request.Kind.Kind {
		return false
	}

	if request.Object.Raw != nil {
		meta := parseMetadataFromObject(request.Object.Raw)
		name := parseNameFromMetadata(meta)

		if policyResource.Name != nil && *policyResource.Name != name {
			return false
		}

		if policyResource.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policyResource.Selector)

			if err != nil {
				// TODO: log that selector is invalid
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

func parseMetadataFromObject(bytes []byte) map[string]interface{} {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	return objectJSON["metadata"].(map[string]interface{})
}

func parseLabelsFromMetadata(meta map[string]interface{}) labels.Set {
	if interfaceMap, ok := meta["labels"].(map[string]interface{}); ok {
		labelMap := make(labels.Set, len(interfaceMap))

		for key, value := range interfaceMap {
			labelMap[key] = value.(string)
		}
		return labelMap
	}
	return nil
}

func parseNameFromMetadata(meta map[string]interface{}) string {
	if name, ok := meta["name"].(string); ok {
		return name
	}
	return ""
}
