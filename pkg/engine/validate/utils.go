package validate

import (
	"container/list"
	"sort"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
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

	keys := make([]string, 0, len(resources))
	for k := range resources {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := resources[k]
		if anchor.IsGlobal(anchor.Parse(k)) {
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
		if a := anchor.Parse(key); anchor.IsCondition(a) || anchor.IsExistence(a) || anchor.IsEquality(a) || anchor.IsNegation(a) || anchor.IsGlobal(a) {
			result[key] = value
		}
	}
	return result
}
