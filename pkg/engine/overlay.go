package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	jsonpatch "github.com/evanphx/json-patch"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
)

// processOverlay processes validation patterns on the resource
func processOverlay(rule kyverno.Rule, resource unstructured.Unstructured) (response RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	glog.V(4).Infof("started applying overlay rule %q (%v)", rule.Name, startTime)
	response.Name = rule.Name
	response.Type = Mutation.String()
	defer func() {
		response.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying overlay rule %q (%v)", response.Name, response.RuleStats.ProcessingTime)
	}()

	patches, overlayerr := processOverlayPatches(resource.UnstructuredContent(), rule.Mutation.Overlay)
	// resource does not satisfy the overlay pattern, we don't apply this rule
	if !reflect.DeepEqual(overlayerr, overlayError{}) {
		switch overlayerr.statusCode {
		// condition key is not present in the resource, don't apply this rule
		// consider as success
		case conditionNotPresent:
			glog.V(3).Infof("Skip applying rule '%s' on resource '%s/%s/%s': %s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), overlayerr.ErrorMsg())
			response.Success = true
			return response, resource
		// conditions are not met, don't apply this rule
		case conditionFailure:
			glog.V(3).Infof("Skip applying rule '%s' on resource '%s/%s/%s': %s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), overlayerr.ErrorMsg())
			//TODO: send zero response and not consider this as applied?
			response.Success = true
			response.Message = overlayerr.ErrorMsg()
			return response, resource
		// rule application failed
		case overlayFailure:
			glog.Errorf("Resource %s/%s/%s: failed to process overlay: %v in the rule %s", resource.GetKind(), resource.GetNamespace(), resource.GetName(), overlayerr.ErrorMsg(), rule.Name)
			response.Success = false
			response.Message = fmt.Sprintf("failed to process overlay: %v", overlayerr.ErrorMsg())
			return response, resource
		default:
			glog.Errorf("Resource %s/%s/%s: Unknown type of error: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), overlayerr.Error())
			response.Success = false
			response.Message = fmt.Sprintf("Unknown type of error: %v", overlayerr.Error())
			return response, resource
		}
	}

	if len(patches) == 0 {
		response.Success = true
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
		msg := fmt.Sprintf("failed to apply JSON patches: %v", err)
		glog.V(2).Infof("%s, patches=%s", msg, string(JoinPatches(patches)))
		response.Success = false
		response.Message = msg
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
	response.Message = fmt.Sprintf("successfully processed overlay")
	response.Patches = patches
	// apply the patches to the resource
	return response, patchedResource
}

func processOverlayPatches(resource, overlay interface{}) ([][]byte, overlayError) {
	if path, overlayerr := meetConditions(resource, overlay); !reflect.DeepEqual(overlayerr, overlayError{}) {
		switch overlayerr.statusCode {
		// anchor key does not exist in the resource, skip applying policy
		case conditionNotPresent:
			glog.V(4).Infof("Mutate rule: skip applying policy: %v at %s", overlayerr, path)
			return nil, newOverlayError(overlayerr.statusCode, fmt.Sprintf("Policy not applied, condition tag not present: %v at %s", overlayerr.ErrorMsg(), path))
		// anchor key is not satisfied in the resource, skip applying policy
		case conditionFailure:
			// anchor key is not satisfied in the resource, skip applying policy
			glog.V(4).Infof("Mutate rule: failed to validate condition at %s, err: %v", path, overlayerr)
			return nil, newOverlayError(overlayerr.statusCode, fmt.Sprintf("Policy not applied, conditions are not met at %s, %v", path, overlayerr))
		}
	}

	patchBytes, err := mutateResourceWithOverlay(resource, overlay)
	if err != nil {
		return patchBytes, newOverlayError(overlayFailure, err.Error())
	}

	return patchBytes, overlayError{}
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
		if anchor.IsConditionAnchor(key) {
			continue
		}

		noAnchorKey := removeAnchor(key)
		currentPath := path + noAnchorKey + "/"
		resourcePart, ok := resourceMap[noAnchorKey]

		if ok && !anchor.IsAddingAnchor(key) {
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

	path = preparePath(path)
	value := prepareJSONValue(overlay)
	patchStr := fmt.Sprintf(`{ "op": "%s", "path": "%s", "value":%s }`, op, path, value)

	// explicitly handle boolean type in annotation
	// keep the type boolean as it is in any other fields
	if strings.Contains(path, "/metadata/annotations") {
		patchStr = wrapBoolean(patchStr)
	}

	// check the patch
	_, err := jsonpatch.DecodePatch([]byte("[" + patchStr + "]"))
	if err != nil {
		glog.V(3).Info(err)
		return nil, fmt.Errorf("Failed to make '%s' patch from an overlay '%s' for path %s, err: %v", op, value, path, err)
	}

	return []byte(patchStr), nil
}

func preparePath(path string) string {
	if path == "" {
		path = "/"
	}

	annPath := "/metadata/annotations/"
	// escape slash in annotation patch
	if strings.Contains(path, annPath) {
		p := path[len(annPath):]
		path = annPath + strings.ReplaceAll(p, "/", "~1")
	}
	return path
}

// converts overlay to JSON string to be inserted into the JSON Patch
func prepareJSONValue(overlay interface{}) string {
	var err error
	// Need to remove anchors from the overlay struct
	overlayWithoutAnchors := removeAnchorFromSubTree(overlay)
	jsonOverlay, err := json.Marshal(overlayWithoutAnchors)
	if err != nil || hasOnlyAnchors(overlay) {
		glog.V(3).Info(err)
		return ""
	}

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

func wrapBoolean(patchStr string) string {
	reTrue := regexp.MustCompile(`:\s*true\s*`)
	if idx := reTrue.FindStringIndex(patchStr); len(idx) != 0 {
		return fmt.Sprintf("%s:\"true\"%s", patchStr[:idx[0]], patchStr[idx[1]:])
	}

	reFalse := regexp.MustCompile(`:\s*false\s*`)
	if idx := reFalse.FindStringIndex(patchStr); len(idx) != 0 {
		return fmt.Sprintf("%s:\"false\"%s", patchStr[:idx[0]], patchStr[idx[1]:])
	}
	return patchStr
}
