package mutation

import (
	"encoding/json"
	"strings"

	"github.com/minio/minio/pkg/wildcard"
	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func ParseMetadataFromObject(bytes []byte) map[string]interface{} {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	return objectJSON["metadata"].(map[string]interface{})
}

func ParseKindFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	return objectJSON["kind"].(string)
}

func ParseLabelsFromMetadata(meta map[string]interface{}) labels.Set {
	if interfaceMap, ok := meta["labels"].(map[string]interface{}); ok {
		labelMap := make(labels.Set, len(interfaceMap))

		for key, value := range interfaceMap {
			labelMap[key] = value.(string)
		}
		return labelMap
	}
	return nil
}

func ParseNameFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if name, ok := meta["name"].(string); ok {
		return name
	}
	return ""
}

func ParseNamespaceFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if namespace, ok := meta["namespace"].(string); ok {
		return namespace
	}
	return ""
}

// returns true if policyResourceName is a regexp
func ParseRegexPolicyResourceName(policyResourceName string) (string, bool) {
	regex := strings.Split(policyResourceName, "regex:")
	if len(regex) == 1 {
		return regex[0], false
	}
	return strings.Trim(regex[1], " "), true
}

// ResourceMeetsRules checks requests kind, name and labels to fit the policy
func ResourceMeetsRules(resourceRaw []byte, description kubepolicy.ResourceDescription, gvk metav1.GroupVersionKind) (bool, error) {
	if description.Kind != gvk.Kind {
		return false, nil
	}

	if resourceRaw != nil {
		meta := ParseMetadataFromObject(resourceRaw)
		name := ParseNameFromObject(resourceRaw)

		if description.Name != nil {

			if !wildcard.Match(*description.Name, name) {
				return false, nil
			}
		}

		if description.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(description.Selector)

			if err != nil {
				return false, err
			}

			labelMap := ParseLabelsFromMetadata(meta)

			if !selector.Matches(labelMap) {
				return false, nil
			}

		}
	}
	return true, nil
}
