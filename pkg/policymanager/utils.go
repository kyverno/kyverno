package policymanager

import (
	"encoding/json"
	"strings"

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
