package anchor

import (
	"path"
	"strings"
)

// GetAnchorsResourcesFromMap returns maps of anchors and resources
func GetAnchorsResourcesFromMap(patternMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := map[string]interface{}{}
	resources := map[string]interface{}{}
	for key, value := range patternMap {
		if a := Parse(key); IsCondition(a) || IsExistence(a) || IsEquality(a) || IsNegation(a) {
			anchors[key] = value
		} else {
			resources[key] = value
		}
	}
	return anchors, resources
}

// RemoveAnchorsFromPath removes all anchor from path string
func RemoveAnchorsFromPath(str string) string {
	parts := strings.Split(str, "/")
	if parts[0] == "" {
		parts = parts[1:]
	}
	for i, part := range parts {
		if a := Parse(part); a != nil {
			parts[i] = a.Key()
		} else {
			parts[i] = part
		}
	}
	newPath := path.Join(parts...)
	if path.IsAbs(str) {
		newPath = "/" + newPath
	}
	return newPath
}

// resourceHasValueForKey checks if a resource has value for a given key
func resourceHasValueForKey(resource interface{}, key string) bool {
	switch typed := resource.(type) {
	case map[string]interface{}:
		if _, ok := typed[key]; ok {
			return true
		}
		return false
	case []interface{}:
		for _, value := range typed {
			if resourceHasValueForKey(value, key) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
