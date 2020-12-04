package wildcards

import (
	commonAnchor "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/minio/minio/pkg/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// ReplaceInSelector replaces label selector keys and values containing
// wildcard characters with matching keys and values from the resource labels.
func ReplaceInSelector(labelSelector *metav1.LabelSelector, resourceLabels map[string]string) {
	result := replaceWildcardsInMap(labelSelector.MatchLabels, resourceLabels)
	labelSelector.MatchLabels = result
}

func replaceWildcardsInMap(patternMap map[string]string, resourceMap map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range patternMap {
		if hasWildcards(k) || hasWildcards(v) {
			matchK, matchV := expandWildcards(k, v, resourceMap, true)
			result[matchK] = matchV
		} else {
			result[k] = v
		}
	}

	return result
}

func hasWildcards(s string) bool {
	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

func expandWildcards(k, v string, resourceMap map[string]string, replace bool) (key string, val string) {
	for k1, v1 := range resourceMap {
		if wildcard.Match(k, k1) {
			if wildcard.Match(v, v1) {
				return k1, v1
			}
		}
	}

	if replace {
		k = replaceWildCardChars(k)
		v = replaceWildCardChars(v)
	}

	return k, v
}

// replaceWildCardChars will replace '*' and '?' characters which are not
// supported by Kubernetes with a '0'.
func replaceWildCardChars(s string) string {
	s = strings.Replace(s, "*", "0", -1)
	s = strings.Replace(s, "?", "0", -1)
	return s
}

// ExpandInMetadata substitutes wildcard characters in map keys for metadata.labels and
// metadata.annotations that are present in a validation pattern. Values are not substituted
// here, as they are evaluated separately while processing the validation pattern.
func ExpandInMetadata(patternMap, resourceMap map[string]interface{}) map[string]interface{} {

	patternMetadata := patternMap["metadata"]
	if patternMetadata == nil {
		return patternMap
	}

	resourceMetadata := resourceMap["metadata"]
	if resourceMetadata == nil {
		return patternMap
	}

	metadata := patternMetadata.(map[string]interface{})
	labels := expandWildcardsInTag("labels", patternMetadata, resourceMetadata)
	if labels != nil {
		metadata["labels"] = labels
	}

	annotations := expandWildcardsInTag("annotations", patternMetadata, resourceMetadata)
	if annotations != nil {
		metadata["annotations"] = annotations
	}

	return patternMap
}

func expandWildcardsInTag(tag string, patternMetadata, resourceMetadata interface{}) map[string]interface{} {
	patternData := getValueAsStringMap(tag, patternMetadata)
	if patternData == nil {
		return nil
	}

	resourceData := getValueAsStringMap(tag, resourceMetadata)
	if resourceData == nil {
		return nil
	}

	results := map[string]interface{}{}
	for k, v := range patternData {
		if hasWildcards(k) {
			newKey := commonAnchor.RemoveAnchor(k)
			matchK, _ := expandWildcards(newKey, v, resourceData, false)
			matchK = strings.Replace(k, newKey, matchK, 1)
			results[matchK] = v
		} else {
			results[k] = v
		}
	}

	return results
}

func getValueAsStringMap(key string, dataMap interface{}) map[string]string {
	if dataMap == nil {
		return nil
	}

	val := dataMap.(map[string]interface{})[key]
	if val == nil {
		return nil
	}

	result := map[string]string{}
	for k, v := range val.(map[string]interface{}) {
		result[k] = v.(string)
	}

	return result
}
