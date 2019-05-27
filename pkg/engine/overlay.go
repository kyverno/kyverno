package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProcessOverlay handles validating admission request
// Checks the target resourse for rules defined in the policy
func ProcessOverlay(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, error) {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	var appliedPatches []PatchBytes

	for _, rule := range policy.Spec.Rules {
		if rule.Mutation == nil || rule.Mutation.Overlay == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			log.Printf("Rule \"%s\" is not applicable to resource\n", rule.Name)
			continue
		}

		overlay := *rule.Mutation.Overlay
		patch, err := applyOverlay(resource, overlay, "/")
		if err != nil {
			return nil, fmt.Errorf("Overlay application failed: %v", err.Error())
		}

		appliedPatches = append(appliedPatches, patch...)
	}

	return appliedPatches, nil
}

// goes down through overlay and resource trees and applies overlay
func applyOverlay(resource, overlay interface{}, path string) ([]PatchBytes, error) {
	var appliedPatches []PatchBytes

	// resource item exists but has different type - replace
	// all subtree within this path by overlay
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		patch, err := replaceSubtree(overlay, path)
		if err != nil {
			return nil, err
		}

		appliedPatches = append(appliedPatches, patch)
		return appliedPatches, nil
	}

	switch typedOverlay := overlay.(type) {
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})

		for key, value := range typedOverlay {
			if wrappedWithParentheses(key) {
				continue
			}
			currentPath := path + key + "/"
			resourcePart, ok := typedResource[key]

			if ok {
				patches, err := applyOverlay(resourcePart, value, currentPath)
				if err != nil {
					return nil, err
				}

				appliedPatches = append(appliedPatches, patches...)

			} else {
				patch, err := insertSubtree(value, currentPath)
				if err != nil {
					return nil, err
				}

				appliedPatches = append(appliedPatches, patch)
			}
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		patches, err := applyOverlayToArray(typedResource, typedOverlay, path)
		if err != nil {
			return nil, err
		}

		appliedPatches = append(appliedPatches, patches...)
	case string, float64, int64:
		patch, err := replaceSubtree(overlay, path)
		if err != nil {
			return nil, err
		}

		appliedPatches = append(appliedPatches, patch)
	default:
		return nil, fmt.Errorf("Overlay has unsupported type: %T", overlay)
	}

	return appliedPatches, nil
}

// for each overlay and resource array elements and applies overlay
func applyOverlayToArray(resource, overlay []interface{}, path string) ([]PatchBytes, error) {
	var appliedPatches []PatchBytes
	if len(overlay) == 0 {
		return nil, fmt.Errorf("overlay does not support empty arrays")
	}

	if len(resource) == 0 {
		patches, err := fillEmptyArray(overlay, path)
		if err != nil {
			return nil, err
		}

		return patches, nil
	}

	if reflect.TypeOf(resource[0]) != reflect.TypeOf(overlay[0]) {
		return nil, fmt.Errorf("overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
	}

	switch overlay[0].(type) {
	case map[string]interface{}:
		for _, overlayElement := range overlay {
			typedOverlay := overlayElement.(map[string]interface{})
			anchors := GetAnchorsFromMap(typedOverlay)
			if len(anchors) > 0 {
				for i, resourceElement := range resource {
					typedResource := resourceElement.(map[string]interface{})

					currentPath := path + strconv.Itoa(i) + "/"
					if !skipArrayObject(typedResource, anchors) {
						patches, err := applyOverlay(resourceElement, overlayElement, currentPath)
						if err != nil {
							return nil, err
						}

						appliedPatches = append(appliedPatches, patches...)
					}

				}
			} else if hasNestedAnchors(overlayElement) {
				for i, resourceElement := range resource {
					currentPath := path + strconv.Itoa(i) + "/"
					patches, err := applyOverlay(resourceElement, overlayElement, currentPath)
					if err != nil {
						return nil, err
					}
					appliedPatches = append(appliedPatches, patches...)
				}
			} else {
				currentPath := path + "0/"
				patch, err := insertSubtree(overlayElement, currentPath)
				if err != nil {
					return nil, err
				}
				appliedPatches = append(appliedPatches, patch)
			}
		}
	default:
		path += "0/"
		for _, value := range overlay {
			patch, err := insertSubtree(value, path)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patch)
		}
	}

	return appliedPatches, nil
}

// In case of empty resource array
// append all non-anchor items to front
func fillEmptyArray(overlay []interface{}, path string) ([]PatchBytes, error) {
	var appliedPatches []PatchBytes
	if len(overlay) == 0 {
		return nil, fmt.Errorf("overlay does not support empty arrays")
	}

	path += "0/"

	switch overlay[0].(type) {
	case map[string]interface{}:
		for _, overlayElement := range overlay {
			typedOverlay := overlayElement.(map[string]interface{})
			anchors := GetAnchorsFromMap(typedOverlay)

			if len(anchors) == 0 {
				patch, err := insertSubtree(overlayElement, path)
				if err != nil {
					return nil, err
				}

				appliedPatches = append(appliedPatches, patch)
			}
		}
	default:
		for _, overlayElement := range overlay {
			patch, err := insertSubtree(overlayElement, path)
			if err != nil {
				return nil, err
			}

			appliedPatches = append(appliedPatches, patch)
		}
	}

	return appliedPatches, nil
}

// Checks if array object matches anchors. If not - skip - return true
func skipArrayObject(object, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := object[key]
		if !ok {
			return true
		}

		if !ValidateValueWithPattern(value, pattern) {
			return true
		}
	}

	return false
}

func insertSubtree(overlay interface{}, path string) ([]byte, error) {
	return processSubtree(overlay, path, "add")
}

func replaceSubtree(overlay interface{}, path string) ([]byte, error) {
	return processSubtree(overlay, path, "replace")
}

func processSubtree(overlay interface{}, path string, op string) ([]byte, error) {
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	if path == "" {
		path = "/"
	}

	value := prepareJSONValue(overlay)
	patchStr := fmt.Sprintf(`{ "op": "%s", "path": "%s", "value": %s }`, op, path, value)

	// check the patch
	_, err := jsonpatch.DecodePatch([]byte("[" + patchStr + "]"))
	if err != nil {
		return nil, err
	}

	return []byte(patchStr), nil
}

// TODO: Overlay is already in JSON, remove this code
// converts overlay to JSON string to be inserted into the JSON Patch
func prepareJSONValue(overlay interface{}) string {
	switch typed := overlay.(type) {
	case map[string]interface{}:
		if len(typed) == 0 {
			return ""
		}

		if hasOnlyAnchors(overlay) {
			return ""
		}

		result := ""
		for key, value := range typed {
			jsonValue := prepareJSONValue(value)

			pair := fmt.Sprintf(`"%s":%s`, key, jsonValue)

			if result != "" {
				result += ", "
			}

			result += pair
		}

		result = fmt.Sprintf(`{ %s }`, result)
		return result
	case []interface{}:
		if len(typed) == 0 {
			return ""
		}

		if hasOnlyAnchors(overlay) {
			return ""
		}

		result := ""
		for _, value := range typed {
			jsonValue := prepareJSONValue(value)

			if result != "" {
				result += ", "
			}

			result += jsonValue
		}

		result = fmt.Sprintf(`[ %s ]`, result)
		return result
	case string:
		return fmt.Sprintf(`"%s"`, typed)
	case float64:
		return fmt.Sprintf("%f", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	default:
		return ""
	}
}

func hasOnlyAnchors(overlay interface{}) bool {
	switch typed := overlay.(type) {
	case map[string]interface{}:
		if anchors := GetAnchorsFromMap(typed); len(anchors) == len(typed) {
			return true
		}

		for _, value := range typed {
			if !hasOnlyAnchors(value) {
				return false
			}
		}

		return true
	default:
		return false
	}
}

func hasNestedAnchors(overlay interface{}) bool {
	switch typed := overlay.(type) {
	case map[string]interface{}:
		if anchors := GetAnchorsFromMap(typed); len(anchors) > 0 {
			return true
		}

		for _, value := range typed {
			if hasNestedAnchors(value) {
				return true
			}
		}
		return false
	case []interface{}:
		for _, value := range typed {
			if hasNestedAnchors(value) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
