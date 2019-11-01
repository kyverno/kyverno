package engine

import (
	"reflect"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
)

func meetConditions(resource, overlay interface{}) bool {
	// overlay has no anchor, return true
	if !hasNestedAnchors(overlay) {
		return true
	}

	// resource item exists but has different type
	// return false if anchor exists in overlay
	// conditon never be true in this case
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		if hasNestedAnchors(overlay) {
			glog.Errorf("Found anchor on different types of element: overlay %T, %v, resource %T, %v\nSkip processing overlay.", overlay, overlay, resource, resource)
			return false
		}
		return true
	}

	return checkConditions(resource, overlay)
}

// resource and overlay should be the same type
func checkConditions(resource, overlay interface{}) bool {
	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		return checkConditionOnMap(typedResource, typedOverlay)
	case []interface{}:
		typedResource := resource.([]interface{})
		return checkConditionOnArray(typedResource, typedOverlay)
	default:
		// anchor on non map/array is invalid:
		// - anchor defined on values
		return true
	}
}

// compareOverlay compare values in anchormap and resourcemap
// i.e. check if B1 == B2
// overlay - (A): B1
// resource - A: B2
func compareOverlay(resource, overlay interface{}) bool {
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		glog.Errorf("Found anchor on different types of element: overlay %T, resource %T\nSkip processing overlay.", overlay, resource)
		return false
	}

	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		for key, overlayVal := range typedOverlay {
			noAnchorKey := removeAnchor(key)
			resourceVal, ok := typedResource[noAnchorKey]
			if !ok {
				return false
			}
			if !compareOverlay(resourceVal, overlayVal) {
				return false
			}
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		for _, overlayElement := range typedOverlay {
			for _, resourceElement := range typedResource {
				if !compareOverlay(resourceElement, overlayElement) {
					return false
				}
			}
		}
	case string, float64, int, int64, bool, nil:
		if !ValidateValueWithPattern(resource, overlay) {
			glog.Errorf("Mutate rule failed validating value %v with overlay %v", resource, overlay)
			return false
		}
	default:
		glog.Errorf("Mutate overlay has unknown type %T, value %v", overlay, overlay)
		return false
	}

	return true
}

func checkConditionOnMap(resourceMap, overlayMap map[string]interface{}) bool {
	anchors, overlayWithoutAnchor := getAnchorAndElementsFromMap(overlayMap)

	if !validateConditionAnchorMap(resourceMap, anchors) {
		return false
	}

	if !validateNonAnchorOverlayMap(resourceMap, overlayWithoutAnchor) {
		return false
	}

	// empty overlayMap
	return true
}

func validateConditionAnchorMap(resourceMap, anchors map[string]interface{}) bool {
	for key, overlayValue := range anchors {
		// skip if key does not have condition anchor
		if !anchor.IsConditionAnchor(key) {
			continue
		}

		// validate condition anchor map
		noAnchorKey := removeAnchor(key)
		if resourceValue, ok := resourceMap[noAnchorKey]; ok {
			if !compareOverlay(resourceValue, overlayValue) {
				return false
			}
		} else {
			// noAnchorKey doesn't exist in resource
			return false
		}
	}
	return true
}

func validateNonAnchorOverlayMap(resourceMap, overlayWithoutAnchor map[string]interface{}) bool {
	// validate resource map (anchors could exist in resource)
	for key, overlayValue := range overlayWithoutAnchor {
		resourceValue, ok := resourceMap[key]
		if !ok {
			// policy: 		"(image)": "*:latest",
			//				"imagePullPolicy": "IfNotPresent",
			// resource:	"(image)": "*:latest",
			// the above case should be allowed
			continue
		}
		if !meetConditions(resourceValue, overlayValue) {
			return false
		}
	}
	return true
}

func checkConditionOnArray(resource, overlay []interface{}) bool {
	if 0 == len(resource) {
		return false
	}

	if 0 == len(overlay) {
		return true
	}

	if reflect.TypeOf(resource[0]) != reflect.TypeOf(overlay[0]) {
		glog.Warningf("Overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
		return false
	}

	return checkConditionsOnArrayOfSameTypes(resource, overlay)
}

func checkConditionsOnArrayOfSameTypes(resource, overlay []interface{}) bool {
	switch overlay[0].(type) {
	case map[string]interface{}:
		return checkConditionsOnArrayOfMaps(resource, overlay)
	default:
		// TODO: array of array?
		glog.Warningf("Anchors not supported in overlay of array type %T\n", overlay[0])
		return false
	}
}

func checkConditionsOnArrayOfMaps(resource, overlay []interface{}) bool {
	for _, overlayElement := range overlay {
		for _, resourceMap := range resource {
			if !checkConditionOnMap(resourceMap.(map[string]interface{}), overlayElement.(map[string]interface{})) {
				return false
			}
		}
	}
	return true
}
