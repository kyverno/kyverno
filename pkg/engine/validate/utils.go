package validate

import (
	"container/list"

	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor"
)

// Checks if pattern has anchors
func hasNestedAnchors(pattern interface{}) bool {
	switch typed := pattern.(type) {
	case map[string]interface{}:
		if anchors := getAnchorsFromMap(typed); len(anchors) > 0 {
			return true
		}
		for _, value := range typed {
			if hasNestedAnchors(value) {
				return true
			}
		}
		return false
	case []interface{}:
		for _, value := range typed {
			if hasNestedAnchors(value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// getSortedNestedAnchorResource - sorts anchors key
func getSortedNestedAnchorResource(resources map[string]interface{}) *list.List {
	sortedResourceKeys := list.New()
	for k, v := range resources {
		if commonAnchors.IsGlobalAnchor(k) {
			sortedResourceKeys.PushFront(k)
			continue
		}
		if hasNestedAnchors(v) {
			sortedResourceKeys.PushFront(k)
		} else {
			sortedResourceKeys.PushBack(k)
		}
	}
	return sortedResourceKeys
}

// getAnchorsFromMap gets the anchor map
func getAnchorsFromMap(anchorsMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range anchorsMap {
		if commonAnchors.IsConditionAnchor(key) || commonAnchors.IsExistenceAnchor(key) || commonAnchors.IsEqualityAnchor(key) || commonAnchors.IsNegationAnchor(key) || commonAnchors.IsGlobalAnchor(key) {
			result[key] = value
		}
	}
	return result
}
