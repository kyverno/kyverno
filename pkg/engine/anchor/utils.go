package anchor

import (
	"path"
	"strings"
)

// GetAnchorsResourcesFromMap returns map of anchors
func GetAnchorsResourcesFromMap(patternMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := map[string]interface{}{}
	resources := map[string]interface{}{}
	for key, value := range patternMap {
		if a := Parse(key); IsCondition(a) || IsExistence(a) || IsEquality(a) || IsNegation(a) {
			anchors[key] = value
			continue
		}
		resources[key] = value
	}
	return anchors, resources
}

// RemoveAnchorsFromPath removes all anchor from path string
func RemoveAnchorsFromPath(str string) string {
	components := strings.Split(str, "/")
	if components[0] == "" {
		components = components[1:]
	}
	for i, component := range components {
		components[i], _ = removeAnchor(component)
	}
	newPath := path.Join(components...)
	if path.IsAbs(str) {
		newPath = "/" + newPath
	}
	return newPath
}
