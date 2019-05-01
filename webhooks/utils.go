package webhooks

import (
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
)

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

func parseNamespaceFromMetadata(meta map[string]interface{}) string {
	if namespace, ok := meta["namespace"].(string); ok {
		return namespace
	}
	return ""
}

// returns true if policyResourceName is a regexp
func parseRegexPolicyResourceName(policyResourceName string) (string, bool) {
	regex := strings.Split(policyResourceName, "regex:")
	if len(regex) == 1 {
		return regex[0], false
	}
	return strings.Trim(regex[1], " "), true
}
