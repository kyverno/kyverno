package mutate

import (
	"bytes"

	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor/common"
)

type buffer struct {
	*bytes.Buffer
}

func (buff buffer) UnmarshalJSON(b []byte) error {
	buff.Reset()
	buff.Write(b)
	return nil
}

func (buff buffer) MarshalJSON() ([]byte, error) {
	return buff.Bytes(), nil
}

// removeAnchor remove special characters around anchored key
func removeAnchor(key string) string {
	if commonAnchors.IsConditionAnchor(key) {
		return key[1 : len(key)-1]
	}

	if commonAnchors.IsExistenceAnchor(key) || commonAnchors.IsAddingAnchor(key) || commonAnchors.IsEqualityAnchor(key) || commonAnchors.IsNegationAnchor(key) || commonAnchors.IsGlobalAnchor(key) {
		return key[2 : len(key)-1]
	}

	return key
}

// getAnchorAndElementsFromMap gets the condition anchor map and resource map without anchor
func getAnchorAndElementsFromMap(anchorsMap map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	anchors := make(map[string]interface{})
	elementsWithoutanchor := make(map[string]interface{})
	for key, value := range anchorsMap {
		if commonAnchors.IsConditionAnchor(key) {
			anchors[key] = value
		} else if !commonAnchors.IsAddingAnchor(key) {
			elementsWithoutanchor[key] = value
		}
	}

	return anchors, elementsWithoutanchor
}
