package engine

import (
	"reflect"
)

// func processoverlay(rule kubepolicy.Rule, rawResource []byte, gvk metav1.GroupVersionKind) ([][]byte, error) {

// 	var resource interface{}
// 	var appliedPatches [][]byte
// 	err := json.Unmarshal(rawResource, &resource)
// 	if err != nil {
// 		return nil, err
// 	}

// 	patches, err := mutateResourceWithOverlay(resource, *rule.Mutation.Overlay)
// 	if err != nil {
// 		return nil, err
// 	}
// 	appliedPatches = append(appliedPatches, patches...)

// 	return appliedPatches, err
// }

func applyoverlay(resource, overlay interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte
	// resource item exists but has different type - replace
	// all subtree within this path by overlay
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		patch, err := replaceSubtree(overlay, path)
		if err != nil {
			return nil, err
		}

		appliedPatches = append(appliedPatches, patch)
	}

	return applyOverlayForSameTypes(resource, overlay, path)
}

func checkConditions(resource, overlay interface{}, path string) bool {

	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		if !checkConditionOnMap(typedResource, typedOverlay) {
			return false
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		if !checkConditionOnArray(typedResource, typedOverlay) {
			return false
		}
	case string, float64, int64, bool:

	default:
		return false
	}
	return true
}

func checkConditionOnMap(resourceMap, overlayMap map[string]interface{}) bool {
	// _ := getAnchorsFromMap(overlayMap)

	return false
}

func checkConditionOnArray(resource, overlay []interface{}) bool {
	return false
}
