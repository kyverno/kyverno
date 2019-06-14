package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/minio/minio/pkg/wildcard"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ResourceMeetsDescription checks requests kind, name and labels to fit the policy rule
func ResourceMeetsDescription(resourceRaw []byte, description kubepolicy.ResourceDescription, gvk metav1.GroupVersionKind) bool {
	if !findKind(description.Kinds, gvk.Kind) {
		return false
	}

	if resourceRaw != nil {
		meta := parseMetadataFromObject(resourceRaw)
		name := ParseNameFromObject(resourceRaw)

		if description.Name != nil {

			if !wildcard.Match(*description.Name, name) {
				return false
			}
		}

		if description.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(description.Selector)

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

func parseMetadataFromObject(bytes []byte) map[string]interface{} {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	return objectJSON["metadata"].(map[string]interface{})
}

func parseKindFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	return objectJSON["kind"].(string)
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

//ParseNameFromObject extracts resource name from JSON obj
func ParseNameFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if name, ok := meta["name"].(string); ok {
		return name
	}
	return ""
}

// ParseNamespaceFromObject extracts the namespace from the JSON obj
func ParseNamespaceFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if namespace, ok := meta["namespace"].(string); ok {
		return namespace
	}
	return ""
}

// ParseRegexPolicyResourceName returns true if policyResourceName is a regexp
func ParseRegexPolicyResourceName(policyResourceName string) (string, bool) {
	regex := strings.Split(policyResourceName, "regex:")
	if len(regex) == 1 {
		return regex[0], false
	}
	return strings.Trim(regex[1], " "), true
}

func getAnchorsFromMap(anchorsMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range anchorsMap {
		if wrappedWithParentheses(key) {
			result[key] = value
		}
	}

	return result
}

func findKind(kinds []string, kindGVK string) bool {
	for _, kind := range kinds {
		if kind == kindGVK {
			return true
		}
	}
	return false
}

func wrappedWithParentheses(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}

func isAddingAnchor(key string) bool {
	const left = "+("
	const right = ")"

	if len(key) < len(left)+len(right) {
		return false
	}

	return left == key[:len(left)] && right == key[len(key)-len(right):]
}

// Checks if array object matches anchors. If not - skip - return true
func skipArrayObject(object, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := object[key]
		if !ok {
			return true
		}

		if !ValidateValueWithPattern(value, pattern) {
			return true
		}
	}

	return false
}

// removeAnchor remove special characters around anchored key
func removeAnchor(key string) string {
	if wrappedWithParentheses(key) {
		return key[1 : len(key)-1]
	}

	if isAddingAnchor(key) {
		return key[2 : len(key)-1]
	}

	return key
}

// convertToFloat converts string and any other value to float64
func convertToFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case string:
		var err error
		floatValue, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, err
		}

		return floatValue, nil
	case float64:
		return typed, nil
	case int64:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	default:
		return 0, fmt.Errorf("Could not convert %T to float64", value)
	}
}
