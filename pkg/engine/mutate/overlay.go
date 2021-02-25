package mutate

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	commonAnchors "github.com/kyverno/kyverno/pkg/engine/anchor/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ProcessOverlay processes mutation overlay on the resource
func ProcessOverlay(log logr.Logger, ruleName string, overlay interface{}, resource unstructured.Unstructured) (resp response.RuleResponse, patchedResource unstructured.Unstructured) {
	startTime := time.Now()
	logger := log.WithValues("rule", ruleName)
	logger.V(4).Info("started applying overlay rule", "startTime", startTime)
	resp.Name = ruleName
	resp.Type = utils.Mutation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		logger.V(4).Info("finished applying overlay rule", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	patches, overlayerr := processOverlayPatches(logger, resource.UnstructuredContent(), overlay)
	if !reflect.DeepEqual(overlayerr, overlayError{}) {
		switch overlayerr.statusCode {

		case conditionNotPresent:
			logger.V(3).Info("skip applying rule", "reason", "conditionNotPresent")
			resp.Success = true
			return resp, resource

		case conditionFailure:
			logger.V(3).Info("skip applying rule", "reason", "conditionFailure")
			//TODO: send zero response and not consider this as applied?
			resp.Success = true
			resp.Message = overlayerr.ErrorMsg()
			return resp, resource

		case overlayFailure:
			logger.Info("failed to process overlay")
			resp.Success = false
			resp.Message = fmt.Sprintf("failed to process overlay: %v", overlayerr.ErrorMsg())
			return resp, resource

		default:
			logger.Info("failed to process overlay")
			resp.Success = false
			resp.Message = fmt.Sprintf("Unknown type of error: %v", overlayerr.Error())
			return resp, resource
		}
	}

	logger.V(4).Info("processing overlay rule", "patches", len(patches))
	if len(patches) == 0 {
		resp.Success = true
		return resp, resource
	}

	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		resp.Success = false
		logger.Error(err, "failed to marshal resource")
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return resp, resource
	}

	var patchResource []byte
	logger.V(5).Info("applying overlay patches", "patches", string(utils.JoinPatches(patches)))
	patchResource, err = utils.ApplyPatches(resourceRaw, patches)
	if err != nil {
		msg := fmt.Sprintf("failed to apply JSON patches: %v", err)
		resp.Success = false
		resp.Message = msg
		return resp, resource
	}

	logger.V(5).Info("patched resource", "patches", string(patchResource))
	err = patchedResource.UnmarshalJSON(patchResource)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to process JSON patches: %v", err)
		return resp, resource
	}

	// rule application successfully
	resp.Success = true
	resp.Message = fmt.Sprintf("successfully processed overlay")
	resp.Patches = patches

	return resp, patchedResource
}

func processOverlayPatches(log logr.Logger, resource, overlay interface{}) ([][]byte, overlayError) {
	if path, overlayerr := meetConditions(log, resource, overlay); !reflect.DeepEqual(overlayerr, overlayError{}) {
		switch overlayerr.statusCode {
		// anchor key does not exist in the resource, skip applying policy
		case conditionNotPresent:
			log.V(4).Info("skip applying policy", "path", path, "error", overlayerr)
			return nil, newOverlayError(overlayerr.statusCode, fmt.Sprintf("Policy not applied, condition tag not present: %v at %s", overlayerr.ErrorMsg(), path))
		// anchor key is not satisfied in the resource, skip applying policy
		case conditionFailure:
			// anchor key is not satisfied in the resource, skip applying policy
			log.V(4).Info("failed to validate condition", "path", path, "error", overlayerr)
			return nil, newOverlayError(overlayerr.statusCode, fmt.Sprintf("Policy not applied, conditions are not met at %s, %v", path, overlayerr))
		}
	}

	patchBytes, err := MutateResourceWithOverlay(resource, overlay)
	if err != nil {
		return patchBytes, newOverlayError(overlayFailure, err.Error())
	}

	return patchBytes, overlayError{}
}

// MutateResourceWithOverlay is a start of overlaying process
func MutateResourceWithOverlay(resource, pattern interface{}) ([][]byte, error) {
	// It assumes that mutation is started from root, so "/" is passed
	return applyOverlay(resource, pattern, "/")
}

// applyOverlay detects type of current item and goes down through overlay and resource trees applying overlay
func applyOverlay(resource, overlay interface{}, path string) ([][]byte, error) {

	// resource item exists but has different type - replace
	// all subtree within this path by overlay
	if reflect.TypeOf(resource) != reflect.TypeOf(overlay) {
		patch, err := replaceSubtree(overlay, path)
		if err != nil {
			return nil, err
		}

		return [][]byte{patch}, nil
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
		if commonAnchors.IsConditionAnchor(key) {
			continue
		}

		noAnchorKey := removeAnchor(key)
		currentPath := path + noAnchorKey + "/"
		resourcePart, ok := resourceMap[noAnchorKey]

		if ok && !commonAnchors.IsAddingAnchor(key) {
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
		anchors := utils.GetAnchorsFromMap(typedOverlay)

		if len(anchors) > 0 {
			// If we have anchors - choose corresponding resource element and mutate it
			patches, err := applyOverlayWithAnchors(resource, overlayElement, path)
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

func applyOverlayWithAnchors(resource []interface{}, overlay interface{}, path string) ([][]byte, error) {
	var appliedPatches [][]byte

	for i, resourceElement := range resource {
		currentPath := path + strconv.Itoa(i) + "/"
		// currentPath example: /spec/template/spec/containers/3/
		patches, err := applyOverlay(resourceElement, overlay, currentPath)
		if err != nil {
			return nil, err
		}
		appliedPatches = append(appliedPatches, patches...)
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
	if strings.Contains(path, "/metadata/annotations") ||
		strings.Contains(path, "/metadata/labels") {
		patchStr = wrapBoolean(patchStr)
	}

	// check the patch
	_, err := jsonpatch.DecodePatch([]byte("[" + patchStr + "]"))
	if err != nil {
		return nil, fmt.Errorf("Failed to make '%s' patch from an overlay '%s' for path %s, err: %v", op, value, path, err)
	}

	return []byte(patchStr), nil
}

func preparePath(path string) string {
	if path == "" {
		return "/"
	}

	// TODO - handle all map key paths
	// The path for a maps needs to be updated to handle keys with slashes.
	// We currently do this for known map types. Ideally we can check the
	// target schema and generically update for any map type.
	path = replaceSlashes(path, "/metadata/annotations/")
	path = replaceSlashes(path, "/metadata/labels/")
	return path
}

// escape slash in paths for maps (labels, annotations, etc.
func replaceSlashes(path, prefix string) string {
	if !strings.Contains(path, prefix) {
		return path
	}

	idx := strings.Index(path, prefix)
	p := path[idx+len(prefix):]
	path = path[:idx+len(prefix)] + strings.ReplaceAll(p, "/", "~1")
	return path
}

// converts overlay to JSON string to be inserted into the JSON Patch
func prepareJSONValue(overlay interface{}) string {
	var err error
	// Need to remove anchors from the overlay struct
	overlayWithoutAnchors := removeAnchorFromSubTree(overlay)
	jsonOverlay, err := json.Marshal(overlayWithoutAnchors)
	if err != nil || hasOnlyAnchors(overlay) {
		log.Log.Error(err, "failed to marshall withoutanchors or has only anchors")
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
	result := make(map[string]interface{})
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
		if anchors := utils.GetAnchorsFromMap(typed); len(anchors) == len(typed) {
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
		if anchors := utils.GetAnchorsFromMap(typed); len(anchors) > 0 {
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
