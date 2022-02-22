package mutate

import (
	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor"
)

// getAnchorAndElementsFromMap gets the condition anchor map and resource map without anchor
func getAnchorAndElementsFromMap(anchorsMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := make(map[string]interface{})
	elementsWithoutanchor := make(map[string]interface{})
	for key, value := range anchorsMap {
		if commonAnchors.IsConditionAnchor(key) {
			anchors[key] = value
		} else if !commonAnchors.IsAddIfNotPresentAnchor(key) {
			elementsWithoutanchor[key] = value
		}
	}

	return anchors, elementsWithoutanchor
}
