package mutate

import (
	"github.com/nirmata/kyverno/pkg/engine/anchor"
)

// removeAnchor remove special characters around anchored key
func removeAnchor(key string) string {
	if anchor.IsConditionAnchor(key) {
		return key[1 : len(key)-1]
	}

	if anchor.IsExistanceAnchor(key) || anchor.IsAddingAnchor(key) || anchor.IsEqualityAnchor(key) || anchor.IsNegationAnchor(key) {
		return key[2 : len(key)-1]
	}

	return key
}

func getRawKeyIfWrappedWithAttributes(str string) string {
	if len(str) < 2 {
		return str
	}

	if str[0] == '(' && str[len(str)-1] == ')' {
		return str[1 : len(str)-1]
	} else if (str[0] == '$' || str[0] == '^' || str[0] == '+' || str[0] == '=') && (str[1] == '(' && str[len(str)-1] == ')') {
		return str[2 : len(str)-1]
	} else {
		return str
	}
}

// getAnchorAndElementsFromMap gets the condition anchor map and resource map without anchor
func getAnchorAndElementsFromMap(anchorsMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := make(map[string]interface{})
	elementsWithoutanchor := make(map[string]interface{})
	for key, value := range anchorsMap {
		if anchor.IsConditionAnchor(key) {
			anchors[key] = value
		} else if !anchor.IsAddingAnchor(key) {
			elementsWithoutanchor[key] = value
		}
	}

	return anchors, elementsWithoutanchor
}
