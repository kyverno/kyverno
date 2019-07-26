package engine

import (
	"reflect"

	"github.com/golang/glog"
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
			glog.V(3).Infof("Found anchor on different types of element: overlay %T, resource %T\nSkip processing overlay.", overlay, resource)
			return false
		}
		return true
	}

	return checkConditions(resource, overlay)
}

func checkConditions(resource, overlay interface{}) bool {
	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		return checkConditionOnMap(typedResource, typedOverlay)
	case []interface{}:
		typedResource := resource.([]interface{})
		return checkConditionOnArray(typedResource, typedOverlay)
	default:
		return true
	}
}

func checkConditionOnMap(resourceMap, overlayMap map[string]interface{}) bool {
	anchors := getAnchorsFromMap(overlayMap)
	if len(anchors) > 0 {
		if !isConditionMetOnMap(resourceMap, anchors) {
			return false
		}
		return true
	}

	for key, value := range overlayMap {
		resourcePart, ok := resourceMap[key]

		if ok && !isAddingAnchor(key) {
			if !meetConditions(resourcePart, value) {
				return false
			}
		}
	}

	// key does not exist or isAddingAnchor
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
		glog.Warningf("Anchors not supported in overlay of array type %T\n", overlay[0])
		return false
	}
}

func checkConditionsOnArrayOfMaps(resource, overlay []interface{}) bool {
	for _, overlayElement := range overlay {
		typedOverlay := overlayElement.(map[string]interface{})
		anchors, overlayWithoutAnchor := getElementsFromMap(typedOverlay)

		if len(anchors) > 0 {
			if !isConditionMet(resource, anchors) {
				return false
			}
		}

		for key, val := range overlayWithoutAnchor {
			if hasNestedAnchors(val) {
				for _, resourceElement := range resource {
					typedResource := resourceElement.(map[string]interface{})

					if resourcePart, ok := typedResource[key]; ok {
						if !meetConditions(resourcePart, val) {
							return false
						}
					}
				}
			}
		}
	}
	return true
}

func isConditionMet(resource []interface{}, anchors map[string]interface{}) bool {
	for _, resourceElement := range resource {
		typedResource := resourceElement.(map[string]interface{})
		for key, pattern := range anchors {
			key = key[1 : len(key)-1]

			value, ok := typedResource[key]
			if !ok {
				continue
			}

			if !ValidateValueWithPattern(value, pattern) {
				return false
			}
		}
	}
	return true
}

func isConditionMetOnMap(resource, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := resource[key]
		if !ok {
			continue
		}

		if !ValidateValueWithPattern(value, pattern) {
			return false
		}
	}
	return true
}
