package engine

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
	"github.com/nirmata/kyverno/pkg/engine/validate"
)

func meetConditions(resource, overlay interface{}) (string, overlayError) {
	return checkConditions(resource, overlay, "/")
}

// resource and overlay should be the same type
func checkConditions(resource, overlay interface{}, path string) (string, overlayError) {
	// overlay has no anchor, return true
	if !hasNestedAnchors(overlay) {
		return "", overlayError{}
	}

	// resource item exists but has different type
	// return false if anchor exists in overlay
	// conditon never be true in this case
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		if hasNestedAnchors(overlay) {
			glog.V(4).Infof("Found anchor on different types of element at path %s: overlay %T, resource %T", path, overlay, resource)
			return path, newOverlayError(conditionFailure,
				fmt.Sprintf("Found anchor on different types of element at path %s: overlay %T %v, resource %T %v", path, overlay, overlay, resource, resource))

		}
		return "", overlayError{}
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
		return "", overlayError{}
	}
}

func checkConditionOnMap(resourceMap, overlayMap map[string]interface{}, path string) (string, overlayError) {
	anchors, overlayWithoutAnchor := getAnchorAndElementsFromMap(overlayMap)

	// validate resource with conditions
	if newPath, err := validateConditionAnchorMap(resourceMap, anchors, path); !reflect.DeepEqual(err, overlayError{}) {
		return newPath, err
	}

	// traverse overlay pattern to further validate conditions
	if newPath, err := validateNonAnchorOverlayMap(resourceMap, overlayWithoutAnchor, path); !reflect.DeepEqual(err, overlayError{}) {
		return newPath, err
	}

	// empty overlayMap
	return "", overlayError{}
}

func checkConditionOnArray(resource, overlay []interface{}, path string) (string, overlayError) {
	if 0 == len(overlay) {
		glog.Infof("Mutate overlay pattern is empty, path %s", path)
		return "", overlayError{}
	}

	if reflect.TypeOf(resource[0]) != reflect.TypeOf(overlay[0]) {
		glog.V(4).Infof("Overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
		return path, newOverlayError(conditionFailure,
			fmt.Sprintf("Overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0]))
	}

	return checkConditionsOnArrayOfSameTypes(resource, overlay, path)
}

func validateConditionAnchorMap(resourceMap, anchors map[string]interface{}, path string) (string, overlayError) {
	for key, overlayValue := range anchors {
		// skip if key does not have condition anchor
		if !anchor.IsConditionAnchor(key) {
			continue
		}

		// validate condition anchor map
		noAnchorKey := removeAnchor(key)
		curPath := path + noAnchorKey + "/"
		if resourceValue, ok := resourceMap[noAnchorKey]; ok {
			// compare entire resourceValue block
			// return immediately on err since condition fails on this block
			if newPath, err := compareOverlay(resourceValue, overlayValue, curPath); !reflect.DeepEqual(err, overlayError{}) {
				return newPath, err
			}
		} else {
			// noAnchorKey doesn't exist in resource
			return curPath, newOverlayError(conditionNotPresent, fmt.Sprintf("resource field is not present %s", noAnchorKey))
		}
	}
	return "", overlayError{}
}

// compareOverlay compare values in anchormap and resourcemap
// i.e. check if B1 == B2
// overlay - (A): B1
// resource - A: B2
func compareOverlay(resource, overlay interface{}, path string) (string, overlayError) {
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		glog.V(4).Infof("Found anchor on different types of element: overlay %T, resource %T", overlay, resource)
		return path, newOverlayError(conditionFailure, fmt.Sprintf("Found anchor on different types of element: overlay %T, resource %T", overlay, resource))
	}

	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		for key, overlayVal := range typedOverlay {
			noAnchorKey := removeAnchor(key)
			curPath := path + noAnchorKey + "/"
			resourceVal, ok := typedResource[noAnchorKey]
			if !ok {
				return curPath, newOverlayError(conditionFailure, fmt.Sprintf("Field %s is not present", noAnchorKey))
			}
			if newPath, err := compareOverlay(resourceVal, overlayVal, curPath); !reflect.DeepEqual(err, overlayError{}) {
				return newPath, err
			}
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		for _, overlayElement := range typedOverlay {
			for _, resourceElement := range typedResource {
				if newPath, err := compareOverlay(resourceElement, overlayElement, path); !reflect.DeepEqual(err, overlayError{}) {
					return newPath, err
				}
			}
		}
	case string, float64, int, int64, bool, nil:
		if !validate.ValidateValueWithPattern(resource, overlay) {
			glog.V(4).Infof("Mutate rule: failed validating value %v with overlay %v", resource, overlay)
			return path, newOverlayError(conditionFailure, fmt.Sprintf("Failed validating value %v with overlay %v", resource, overlay))
		}
	default:
		return path, newOverlayError(conditionFailure, fmt.Sprintf("Overlay has unknown type %T, value %v", overlay, overlay))
	}

	return "", overlayError{}
}

// validateNonAnchorOverlayMap validate anchor condition in overlay block without anchor
func validateNonAnchorOverlayMap(resourceMap, overlayWithoutAnchor map[string]interface{}, path string) (string, overlayError) {
	// validate resource map (anchors could exist in resource)
	for key, overlayValue := range overlayWithoutAnchor {
		curPath := path + key + "/"
		resourceValue, ok := resourceMap[key]
		if !ok {
			if !hasNestedAnchors(overlayValue) {
				// policy: 		"(image)": "*:latest",
				//				"imagePullPolicy": "IfNotPresent",
				// resource:	"(image)": "*:latest",
				// the above case should be allowed
				continue
			}
		}
		if newPath, err := checkConditions(resourceValue, overlayValue, curPath); !reflect.DeepEqual(err, overlayError{}) {
			return newPath, err
		}
	}
	return "", overlayError{}
}

func checkConditionsOnArrayOfSameTypes(resource, overlay []interface{}, path string) (string, overlayError) {
	switch overlay[0].(type) {
	case map[string]interface{}:
		return checkConditionsOnArrayOfMaps(resource, overlay, path)
	default:
		for i, overlayElement := range overlay {
			curPath := path + strconv.Itoa(i) + "/"
			path, err := checkConditions(resource[i], overlayElement, curPath)
			if !reflect.DeepEqual(err, overlayError{}) {
				return path, err
			}
		}
	}
	return "", overlayError{}
}

func checkConditionsOnArrayOfMaps(resource, overlay []interface{}, path string) (string, overlayError) {
	var newPath string
	var err overlayError

	for i, overlayElement := range overlay {
		for _, resourceMap := range resource {
			curPath := path + strconv.Itoa(i) + "/"
			newPath, err = checkConditionOnMap(resourceMap.(map[string]interface{}), overlayElement.(map[string]interface{}), curPath)
			// when resource has multiple same blocks of the overlay block
			// return true if there is one resource block meet the overlay pattern
			// reference: TestMeetConditions_AtleastOneExist
			if reflect.DeepEqual(err, overlayError{}) {
				return "", overlayError{}
			}
		}
	}

	// report last error
	return newPath, err
}
