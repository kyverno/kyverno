package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	jsonpatch "github.com/evanphx/json-patch"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

// rawResource handles validating admission request
// Checks the target resources for rules defined in the policy
// TODO: pass in the unstructured object in stead of raw byte?
func processOverlay(rule kyverno.Rule, rawResource []byte) ([][]byte, error) {
	var resource interface{}
	if err := json.Unmarshal(rawResource, &resource); err != nil {
		glog.V(4).Infof("unable to unmarshal resource : %v", err)
		return nil, err
	}

	resourceInfo := ParseResourceInfoFromObject(rawResource)
	patches, err := processOverlayPatches(resource, rule.Mutation.Overlay)
	if err != nil && strings.Contains(err.Error(), "Conditions are not met") {
		// glog.V(4).Infof("overlay pattern %s does not match resource %s/%s", rule.Mutation.Overlay, resourceUnstr.GetNamespace(), resourceUnstr.GetName())
		glog.Infof("Resource does not meet conditions in overlay pattern, resource=%s, rule=%s\n", resourceInfo, rule.Name)
		// patches, err := processOverlayPatches(resource, rule.Mutation.Overlay)
		// if err != nil && strings.Contains(err.Error(), "Conditions are not met") {
		// 	glog.V(4).Infof("overlay pattern %s does not match resource %s/%s", rule.Mutation.Overlay, resourceUnstr.GetNamespace(), resourceUnstr.GetName())
		// 	return nil, nil
	}

	return patches, err
}

// rawResource handles validating admission request
// Checks the target resources for rules defined in the policy
// TODO: pass in the unstructured object in stead of raw byte?
func processOverlayNew(rule kyverno.Rule, resource unstructured.Unstructured) (response RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	glog.V(4).Infof("started applying overlay rule %q (%v)", rule.Name, startTime)
	response.Name = rule.Name
	response.Type = Mutation.String()
	defer func() {
		response.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying overlay rule %q (%v)", response.Name, response.RuleStats.ProcessingTime)
	}()

	patches, err := processOverlayPatches(resource.UnstructuredContent(), rule.Mutation.Overlay)
	// resource does not satisfy the overlay pattern, we dont apply this rule
	if err != nil && strings.Contains(err.Error(), "Conditions are not met") {
		glog.V(4).Infof("Resource %s/%s/%s does not meet the conditions in the rule %s with overlay pattern %s", resource.GetKind(), resource.GetNamespace(), resource.GetName(), rule.Name, rule.Mutation.Overlay)
		//TODO: send zero response and not consider this as applied?
		return RuleResponse{}, resource
	}

	if err != nil {
		// rule application failed
		response.Success = false
		response.Message = fmt.Sprintf("failed to process overlay: %v", err)
		return response, resource
	}
	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		response.Success = false
		glog.Infof("unable to marshall resource: %v", err)
		response.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return response, resource
	}

	var patchResource []byte
	patchResource, err = ApplyPatches(resourceRaw, patches)
	if err != nil {
		glog.Info("failed to apply patch")
		response.Success = false
		response.Message = fmt.Sprintf("failed to apply JSON patches: %v", err)
		return response, resource
	}
	err = patchedResource.UnmarshalJSON(patchResource)
	if err != nil {
		glog.Infof("failed to unmarshall resource to undstructured: %v", err)
		response.Success = false
		response.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return response, resource
	}

	// rule application succesfuly
	response.Success = true
	response.Message = fmt.Sprintf("succesfully process overlay")
	response.Patches = patches
	// apply the patches to the resource
	return response, patchedResource
}
func processOverlayPatches(resource, overlay interface{}) ([][]byte, error) {

	if !meetConditions(resource, overlay) {
		return nil, errors.New("Conditions are not met")
	}

	return mutateResourceWithOverlay(resource, overlay)
}

// mutateResourceWithOverlay is a start of overlaying process
func mutateResourceWithOverlay(resource, pattern interface{}) ([][]byte, error) {
	// It assumes that mutation is started from root, so "/" is passed
	return applyOverlay(resource, pattern, "/")
}

// applyOverlay detects type of current item and goes down through overlay and resource trees applying overlay
func applyOverlay(resource, overlay interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte
	// resource item exists but has different type - replace
	// all subtree within this path by overlay
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		patch, err := replaceSubtree(overlay, path)
		if err != nil {
			return nil, err
		}

		appliedPatches = append(appliedPatches, patch)
		//TODO : check if return is needed ?
	}
	return applyOverlayForSameTypes(resource, overlay, path)
}

// applyOverlayForSameTypes is applyOverlay for cases when TypeOf(resource) == TypeOf(overlay)
func applyOverlayForSameTypes(resource, overlay interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	// detect the type of resource and overlay and select corresponding handler
	switch typedOverlay := overlay.(type) {
	// map
	case map[string]interface{}:
		typedResource := resource.(map[string]interface{})
		patches, err := applyOverlayToMap(typedResource, typedOverlay, path)
		if err != nil {
			return nil, err
		}
		appliedPatches = append(appliedPatches, patches...)
	// array
	case []interface{}:
		typedResource := resource.([]interface{})
		patches, err := applyOverlayToArray(typedResource, typedOverlay, path)
		if err != nil {
			return nil, err
		}
		appliedPatches = append(appliedPatches, patches...)
	// elementary types
	case string, float64, int64, bool:
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

// for each overlay and resource map elements applies overlay
func applyOverlayToMap(resourceMap, overlayMap map[string]interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	for key, value := range overlayMap {
		// skip anchor element because it has condition, not
		// the value that must replace resource value
		if isConditionAnchor(key) {
			continue
		}

		noAnchorKey := removeAnchor(key)
		currentPath := path + noAnchorKey + "/"
		resourcePart, ok := resourceMap[noAnchorKey]

		if ok && !isAddingAnchor(key) {
			// Key exists - go down through the overlay and resource trees
			patches, err := applyOverlay(resourcePart, value, currentPath)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patches...)
		}

		if !ok {
			// Key does not exist - insert entire overlay subtree
			patch, err := insertSubtree(value, currentPath)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patch)
		}
	}

	return appliedPatches, nil
}

// for each overlay and resource array elements applies overlay
func applyOverlayToArray(resource, overlay []interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	if 0 == len(overlay) {
		return nil, errors.New("Empty array detected in the overlay")
	}

	if 0 == len(resource) {
		// If array resource is empty, insert part from overlay
		patch, err := insertSubtree(overlay, path)
		if err != nil {
			return nil, err
		}
		appliedPatches = append(appliedPatches, patch)

		return appliedPatches, nil
	}

	if reflect.TypeOf(resource[0]) != reflect.TypeOf(overlay[0]) {
		return nil, fmt.Errorf("Overlay array and resource array have elements of different types: %T and %T", overlay[0], resource[0])
	}

	return applyOverlayToArrayOfSameTypes(resource, overlay, path)
}

// applyOverlayToArrayOfSameTypes applies overlay to array elements if they (resource and overlay elements) have same type
func applyOverlayToArrayOfSameTypes(resource, overlay []interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	switch overlay[0].(type) {
	case map[string]interface{}:
		return applyOverlayToArrayOfMaps(resource, overlay, path)
	default:
		lastElementIdx := len(resource)

		// Add elements to the end
		for i, value := range overlay {
			currentPath := path + strconv.Itoa(lastElementIdx+i) + "/"
			// currentPath example: /spec/template/spec/containers/3/
			patch, err := insertSubtree(value, currentPath)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patch)
		}
	}

	return appliedPatches, nil
}

// Array of maps needs special handling as far as it can have anchors.
func applyOverlayToArrayOfMaps(resource, overlay []interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	lastElementIdx := len(resource)
	for i, overlayElement := range overlay {
		typedOverlay := overlayElement.(map[string]interface{})
		anchors := getAnchorsFromMap(typedOverlay)

		if len(anchors) > 0 {
			// If we have anchors - choose corresponding resource element and mutate it
			patches, err := applyOverlayWithAnchors(resource, overlayElement, anchors, path)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patches...)
		} else if hasNestedAnchors(overlayElement) {
			// If we have anchors on the lower level - continue traversing overlay and resource trees
			for j, resourceElement := range resource {
				currentPath := path + strconv.Itoa(j) + "/"
				// currentPath example: /spec/template/spec/containers/3/
				patches, err := applyOverlay(resourceElement, overlayElement, currentPath)
				if err != nil {
					return nil, err
				}
				appliedPatches = append(appliedPatches, patches...)
			}
		} else {
			// Overlay subtree has no anchors - insert new element
			currentPath := path + strconv.Itoa(lastElementIdx+i) + "/"
			// currentPath example: /spec/template/spec/containers/3/
			patch, err := insertSubtree(overlayElement, currentPath)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patch)
		}
	}

	return appliedPatches, nil
}

func applyOverlayWithAnchors(resource []interface{}, overlay interface{}, anchors map[string]interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	for i, resourceElement := range resource {
		typedResource := resourceElement.(map[string]interface{})

		currentPath := path + strconv.Itoa(i) + "/"
		// currentPath example: /spec/template/spec/containers/3/
		if !skipArrayObject(typedResource, anchors) {
			patches, err := applyOverlay(resourceElement, overlay, currentPath)
			if err != nil {
				return nil, err
			}
			appliedPatches = append(appliedPatches, patches...)
		}
	}

	return appliedPatches, nil
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
		glog.V(3).Info(err)
		return nil, fmt.Errorf("Failed to make '%s' patch from an overlay '%s' for path %s", op, value, path)
	}

	return []byte(patchStr), nil
}

// converts overlay to JSON string to be inserted into the JSON Patch
func prepareJSONValue(overlay interface{}) string {
	var err error
	if err != nil || hasOnlyAnchors(overlay) {
		glog.V(3).Info(err)
		return ""
	}
	// Need to remove anchors from the overlay struct
	overlayWithoutAnchors := removeAnchorFromSubTree(overlay)
	jsonOverlay, err := json.Marshal(overlayWithoutAnchors)
	return string(jsonOverlay)
}

func removeAnchorFromSubTree(overlay interface{}) interface{} {
	var result interface{}
	switch typed := overlay.(type) {
	case map[string]interface{}:
		// assuming only keys have anchors
		result = removeAnchroFromMap(typed)
	case []interface{}:
		arrayResult := make([]interface{}, 0)
		for _, value := range typed {
			arrayResult = append(arrayResult, removeAnchorFromSubTree(value))
		}
		result = arrayResult
	default:
		result = overlay
	}
	return result
}

func removeAnchroFromMap(overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, 0)
	for k, v := range overlay {
		result[getRawKeyIfWrappedWithAttributes(k)] = removeAnchorFromSubTree(v)
	}
	return result
}

// Anchor has pattern value, so resource shouldn't be mutated with it
// If entire subtree has only anchor keys - we should skip inserting it
func hasOnlyAnchors(overlay interface{}) bool {
	switch typed := overlay.(type) {
	case map[string]interface{}:
		if anchors := getAnchorsFromMap(typed); len(anchors) == len(typed) {
			return true
		}

		for _, value := range typed {
			if !hasOnlyAnchors(value) {
				return false
			}
		}
	case []interface{}:
		for _, value := range typed {
			if !hasOnlyAnchors(value) {
				return false
			}
		}
	default:
		return false
	}

	return true
}

// Checks if subtree has anchors
func hasNestedAnchors(overlay interface{}) bool {
	switch typed := overlay.(type) {
	case map[string]interface{}:
		if anchors := getAnchorsFromMap(typed); len(anchors) > 0 {
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
