package wildcards

import (
	"strings"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplaceInSelector replaces label selector keys and values containing
// wildcard characters with matching keys and values from the resource labels.
func ReplaceInSelector(labelSelector *metav1.LabelSelector, resourceLabels map[string]string) *metav1.LabelSelector {
	labelSelector = labelSelector.DeepCopy()
	result := replaceWildcardsInMapKeyValues(labelSelector.MatchLabels, resourceLabels)
	labelSelector.MatchLabels = result
	return labelSelector
}

// replaceWildcardsInMap will expand  the "key" and "value" and will replace wildcard characters
// It also does not handle anchors as these are not expected in selectors
func replaceWildcardsInMapKeyValues(patternMap map[string]string, resourceMap map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range patternMap {
		if wildcard.ContainsWildcard(k) || wildcard.ContainsWildcard(v) {
			matchK, matchV := expandWildcards(k, v, resourceMap, true, true)
			result[matchK] = matchV
		} else {
			result[k] = v
		}
	}
	return result
}

func expandWildcards(k, v string, resourceMap map[string]string, matchValue, replace bool) (key string, val string) {
	for k1, v1 := range resourceMap {
		if wildcard.Match(k, k1) {
			if !matchValue {
				return k1, v1
			} else if wildcard.Match(v, v1) {
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
	s = strings.ReplaceAll(s, "*", "0")
	s = strings.ReplaceAll(s, "?", "0")
	return s
}

// ExpandInMetadata substitutes wildcard characters in map keys for metadata.labels and
// metadata.annotations that are present in a validation pattern. Values are not substituted
// here, as they are evaluated separately while processing the validation pattern. Anchors
// on the tags (e.g. "=(kubernetes.io/*)" will be preserved when the values are expanded.
func ExpandInMetadata(patternMap, resourceMap map[string]interface{}) map[string]interface{} {
	_, patternMetadata := getPatternValue("metadata", patternMap)
	if patternMetadata == nil {
		return patternMap
	}

	resourceMetadata := resourceMap["metadata"]
	if resourceMetadata == nil {
		return patternMap
	}

	metadata := patternMetadata.(map[string]interface{})
	labelsKey, labels := expandWildcardsInTag("labels", patternMetadata, resourceMetadata)
	if labels != nil {
		metadata[labelsKey] = labels
	}
	annotationsKey, annotations := expandWildcardsInTag("annotations", patternMetadata, resourceMetadata)
	if annotations != nil {
		metadata[annotationsKey] = annotations
	}
	return patternMap
}

func getPatternValue(tag string, pattern map[string]interface{}) (string, interface{}) {
	for k, v := range pattern {
		if k == tag {
			return k, v
		}
		if a := anchor.Parse(k); a != nil && a.Key() == tag {
			return k, v
		}
	}
	return "", nil
}

// expandWildcardsInTag
func expandWildcardsInTag(tag string, patternMetadata, resourceMetadata interface{}) (string, map[string]interface{}) {
	patternKey, patternData := getValueAsStringMap(tag, patternMetadata)
	if patternData == nil {
		return "", nil
	}

	_, resourceData := getValueAsStringMap(tag, resourceMetadata)
	if resourceData == nil {
		return "", nil
	}

	results := replaceWildcardsInMapKeys(patternData, resourceData)
	return patternKey, results
}

func getValueAsStringMap(key string, data interface{}) (string, map[string]string) {
	if data == nil {
		return "", nil
	}

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return "", nil
	}
	patternKey, val := getPatternValue(key, dataMap)

	if val == nil {
		return "", nil
	}

	result := map[string]string{}

	valMap, ok := val.(map[string]interface{})
	if !ok {
		return "", nil
	}

	for k, v := range valMap {
		result[k] = v.(string)
	}

	return patternKey, result
}

// replaceWildcardsInMapKeys will expand only the "key" and not replace wildcard characters in the key or values
// It also preserves anchors in keys
func replaceWildcardsInMapKeys(patternData, resourceData map[string]string) map[string]interface{} {
	results := map[string]interface{}{}
	for k, v := range patternData {
		if wildcard.ContainsWildcard(k) {
			if a := anchor.Parse(k); a != nil {
				matchK, _ := expandWildcards(a.Key(), v, resourceData, false, false)
				results[anchor.String(a.Type(), matchK)] = v
			} else {
				matchK, _ := expandWildcards(k, v, resourceData, false, false)
				results[matchK] = v
			}
		} else {
			results[k] = v
		}
	}
	return results
}
