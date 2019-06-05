package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/nirmata/kyverno/pkg/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProcessOverlay handles validating admission request
// Checks the target resourse for rules defined in the policy
func ProcessOverlay(overlay interface{}, rawResource []byte, gvk metav1.GroupVersionKind) ([]PatchBytes, result.RuleApplicationResult) {
	var resource interface{}
	var appliedPatches []PatchBytes
	json.Unmarshal(rawResource, &resource)

	overlayApplicationResult := result.NewRuleApplicationResult("")
	if overlay == nil {
		return nil, overlayApplicationResult
	}

	patch := applyOverlay(resource, overlay, "/", &overlayApplicationResult)
	if overlayApplicationResult.GetReason() == result.Success {
		appliedPatches = append(appliedPatches, patch...)
	}

	return appliedPatches, overlayApplicationResult
}

// goes down through overlay and resource trees and applies overlay
func applyOverlay(resource, overlay interface{}, path string, res *result.RuleApplicationResult) []PatchBytes {
	var appliedPatches []PatchBytes

	// resource item exists but has different type - replace
	// all subtree within this path by overlay
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		patch := replaceSubtree(overlay, path, res)
		if res.Reason == result.Success {
			appliedPatches = append(appliedPatches, patch)
		}
		return appliedPatches
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
				patches := applyOverlay(resourcePart, value, currentPath, res)
				if res.Reason == result.Success {
					appliedPatches = append(appliedPatches, patches...)
				}

			} else {
				patch := insertSubtree(value, currentPath, res)
				if res.Reason == result.Success {
					appliedPatches = append(appliedPatches, patch)
				}

				appliedPatches = append(appliedPatches, patch)
			}
		}
	case []interface{}:
		typedResource := resource.([]interface{})
		patches := applyOverlayToArray(typedResource, typedOverlay, path, res)
		if res.Reason == result.Success {
			appliedPatches = append(appliedPatches, patches...)
		}
	case string, float64, int64, bool:
		patch := replaceSubtree(overlay, path, res)
		if res.Reason == result.Success {
			appliedPatches = append(appliedPatches, patch)
		}
	default:
		res.FailWithMessagef("Overlay has unsupported type: %T", overlay)
		return nil
	}

	return appliedPatches
}

// for each overlay and resource array elements and applies overlay
func applyOverlayToArray(resource, overlay []interface{}, path string, res *result.RuleApplicationResult) []PatchBytes {
	var appliedPatches []PatchBytes
	if len(overlay) == 0 {
		res.FailWithMessagef("Empty array detected in the overlay")
		return nil
	}

	if len(resource) == 0 {
		return fillEmptyArray(overlay, path, res)
	}

	if reflect.TypeOf(resource[0]) != reflect.TypeOf(overlay[0]) {
		res.FailWithMessagef("overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
		return nil
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
						patches := applyOverlay(resourceElement, overlayElement, currentPath, res)
						if res.Reason == result.Success {
							appliedPatches = append(appliedPatches, patches...)
						}
					}

				}
			} else if hasNestedAnchors(overlayElement) {
				for i, resourceElement := range resource {
					currentPath := path + strconv.Itoa(i) + "/"
					patches := applyOverlay(resourceElement, overlayElement, currentPath, res)
					if res.Reason == result.Success {
						appliedPatches = append(appliedPatches, patches...)
					}
				}
			} else {
				currentPath := path + "0/"
				patch := insertSubtree(overlayElement, currentPath, res)
				if res.Reason == result.Success {
					appliedPatches = append(appliedPatches, patch)
				}
			}
		}
	default:
		path += "0/"
		for _, value := range overlay {
			patch := insertSubtree(value, path, res)
			if res.Reason == result.Success {
				appliedPatches = append(appliedPatches, patch)
			}
		}
	}

	return appliedPatches
}

// In case of empty resource array
// append all non-anchor items to front
func fillEmptyArray(overlay []interface{}, path string, res *result.RuleApplicationResult) []PatchBytes {
	var appliedPatches []PatchBytes
	if len(overlay) == 0 {
		res.FailWithMessagef("Empty array detected in the overlay")
		return nil
	}

	path += "0/"

	switch overlay[0].(type) {
	case map[string]interface{}:
		for _, overlayElement := range overlay {
			typedOverlay := overlayElement.(map[string]interface{})
			anchors := GetAnchorsFromMap(typedOverlay)

			if len(anchors) == 0 {
				patch := insertSubtree(overlayElement, path, res)
				if res.Reason == result.Success {
					appliedPatches = append(appliedPatches, patch)
				}
			}
		}
	default:
		for _, overlayElement := range overlay {
			patch := insertSubtree(overlayElement, path, res)
			if res.Reason == result.Success {
				appliedPatches = append(appliedPatches, patch)
			}
		}
	}

	return appliedPatches
}

func insertSubtree(overlay interface{}, path string, res *result.RuleApplicationResult) []byte {
	return processSubtree(overlay, path, "add", res)
}

func replaceSubtree(overlay interface{}, path string, res *result.RuleApplicationResult) []byte {
	return processSubtree(overlay, path, "replace", res)
}

func processSubtree(overlay interface{}, path string, op string, res *result.RuleApplicationResult) PatchBytes {
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
		res.FailWithMessagef("Failed to make '%s' patch from an overlay '%s' for path %s", op, value, path)
		return nil
	}

	return PatchBytes(patchStr)
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
	case bool:
		return fmt.Sprintf("%t", typed)
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
