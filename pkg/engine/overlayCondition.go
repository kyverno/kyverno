package engine

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
)

func meetConditions(resource, overlay interface{}) (string, error) {
	return checkConditions(resource, overlay, "/")
}

// resource and overlay should be the same type
func checkConditions(resource, overlay interface{}, path string) (string, error) {
	// overlay has no anchor, return true
	if !hasNestedAnchors(overlay) {
		return "", nil
	}

	// resource item exists but has different type
	// return false if anchor exists in overlay
	// conditon never be true in this case
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		if hasNestedAnchors(overlay) {
			glog.V(4).Infof("Found anchor on different types of element at path %s: overlay %T, resource %T", path, overlay, resource)
			return path, fmt.Errorf("Found anchor on different types of element at path %s: overlay %T %v, resource %T %v", path, overlay, overlay, resource, resource)

		}
		return "", nil
	}

	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		return checkConditionOnMap(typedResource, typedOverlay, path)
	case []interface{}:
		typedResource := resource.([]interface{})
		return checkConditionOnArray(typedResource, typedOverlay, path)
	default:
		// anchor on non map/array is invalid:
		// - anchor defined on values
		glog.Warningln("Found invalid conditional anchor: anchor defined on values")
		return "", nil
	}
}

func checkConditionOnMap(resourceMap, overlayMap map[string]interface{}, path string) (string, error) {
	anchors, overlayWithoutAnchor := getAnchorAndElementsFromMap(overlayMap)

	if newPath, err := validateConditionAnchorMap(resourceMap, anchors, path); err != nil {
		return newPath, err
	}

	if newPath, err := validateNonAnchorOverlayMap(resourceMap, overlayWithoutAnchor, path); err != nil {
		return newPath, err
	}

	// empty overlayMap
	return "", nil
}

func checkConditionOnArray(resource, overlay []interface{}, path string) (string, error) {
	if 0 == len(overlay) {
		glog.Infof("Mutate overlay pattern is empty, path %s", path)
		return "", nil
	}

	if reflect.TypeOf(resource[0]) != reflect.TypeOf(overlay[0]) {
		glog.V(4).Infof("Overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
		return path, fmt.Errorf("Overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
	}

	return checkConditionsOnArrayOfSameTypes(resource, overlay, path)
}

func validateConditionAnchorMap(resourceMap, anchors map[string]interface{}, path string) (string, error) {
	for key, overlayValue := range anchors {
		// skip if key does not have condition anchor
		if !anchor.IsConditionAnchor(key) {
			continue
		}

		// validate condition anchor map
		noAnchorKey := removeAnchor(key)
		curPath := path + noAnchorKey + "/"
		if resourceValue, ok := resourceMap[noAnchorKey]; ok {
			if newPath, err := compareOverlay(resourceValue, overlayValue, curPath); err != nil {
				return newPath, err
			}
		} else {
			// noAnchorKey doesn't exist in resource
			return curPath, fmt.Errorf("resource field %s is not present", noAnchorKey)
		}
	}
	return "", nil
}

// compareOverlay compare values in anchormap and resourcemap
// i.e. check if B1 == B2
// overlay - (A): B1
// resource - A: B2
func compareOverlay(resource, overlay interface{}, path string) (string, error) {
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		glog.Errorf("Found anchor on different types of element: overlay %T, resource %T\nSkip processing overlay.", overlay, resource)
		return path, fmt.Errorf("")
	}

	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		for key, overlayVal := range typedOverlay {
			noAnchorKey := removeAnchor(key)
			curPath := path + noAnchorKey + "/"
			resourceVal, ok := typedResource[noAnchorKey]
			if !ok {
				return curPath, fmt.Errorf("Field %s is not present", noAnchorKey)
			}
			if newPath, err := compareOverlay(resourceVal, overlayVal, curPath); err != nil {
				return newPath, err
			}
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		for _, overlayElement := range typedOverlay {
			for _, resourceElement := range typedResource {
				if newPath, err := compareOverlay(resourceElement, overlayElement, path); err != nil {
					return newPath, err
				}
			}
		}
	case string, float64, int, int64, bool, nil:
		if !ValidateValueWithPattern(resource, overlay) {
			glog.V(4).Infof("Mutate rule: failed validating value %v with overlay %v", resource, overlay)
			return path, fmt.Errorf("failed validating value %v with overlay %v", resource, overlay)
		}
	default:
		return path, fmt.Errorf("overlay has unknown type %T, value %v", overlay, overlay)
	}

	return "", nil
}

func validateNonAnchorOverlayMap(resourceMap, overlayWithoutAnchor map[string]interface{}, path string) (string, error) {
	// validate resource map (anchors could exist in resource)
	for key, overlayValue := range overlayWithoutAnchor {
		curPath := path + key + "/"
		resourceValue, ok := resourceMap[key]
		if !ok {
			// policy: 		"(image)": "*:latest",
			//				"imagePullPolicy": "IfNotPresent",
			// resource:	"(image)": "*:latest",
			// the above case should be allowed
			continue
		}
		if newPath, err := checkConditions(resourceValue, overlayValue, curPath); err != nil {
			return newPath, err
		}
	}
	return "", nil
}

func checkConditionsOnArrayOfSameTypes(resource, overlay []interface{}, path string) (string, error) {
	switch overlay[0].(type) {
	case map[string]interface{}:
		return checkConditionsOnArrayOfMaps(resource, overlay, path)
	default:
		for i, overlayElement := range overlay {
			curPath := path + strconv.Itoa(i) + "/"
			path, err := checkConditions(resource[i], overlayElement, curPath)
			if err != nil {
				return path, err
			}
		}
	}
	return "", nil
}

func checkConditionsOnArrayOfMaps(resource, overlay []interface{}, path string) (string, error) {
	for i, overlayElement := range overlay {
		for _, resourceMap := range resource {
			curPath := path + strconv.Itoa(i) + "/"
			if newPath, err := checkConditionOnMap(resourceMap.(map[string]interface{}), overlayElement.(map[string]interface{}), curPath); err != nil {
				return newPath, err
			}
		}
	}
	return "", nil
}
