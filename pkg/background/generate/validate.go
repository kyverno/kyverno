package generate

import (
	"container/list"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/wildcards"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Handler struct {
	element string
	pattern interface{}
	path    string
}

type resourceElementHandler = func(log logr.Logger, resourceElement, patternElement, originPattern interface{}, path string) (string, error)

// ValidateResourceWithPattern is a start of element-by-element validation process
// It assumes that validation is started from root, so "/" is passed
// Anchors are not expected in the pattern
func ValidateResourceWithPattern(log logr.Logger, resource, pattern interface{}) (string, error) {
	elemPath, err := validateResourceElement(log, resource, pattern, pattern, "/")
	if err != nil {
		return elemPath, err
	}
	return "", nil
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(log logr.Logger, resourceElement, patternElement, originPattern interface{}, path string) (string, error) {
	// var err error
	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			log.V(4).Info("Pattern and resource have different structures.", "path", path, "expected", fmt.Sprintf("%T", patternElement), "current", fmt.Sprintf("%T", resourceElement))
			return path, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}
		return validateMap(log, typedResourceElement, typedPatternElement, originPattern, path)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			log.V(4).Info("Pattern and resource have different structures.", "path", path, "expected", fmt.Sprintf("%T", patternElement), "current", fmt.Sprintf("%T", resourceElement))
			return path, fmt.Errorf("Validation rule Failed at path %s, resource does not satisfy the expected overlay pattern", path)
		}
		return validateArray(log, typedResourceElement, typedPatternElement, originPattern, path)
	// elementary values
	case string, float64, int, int64, bool, nil:
		if !common.ValidateValueWithPattern(log, resourceElement, patternElement) {
			return path, fmt.Errorf("value '%v' does not match '%v' at path %s", resourceElement, patternElement, path)
		}

	default:
		log.V(4).Info("Pattern contains unknown type", "path", path, "current", fmt.Sprintf("%T", patternElement))
		return path, fmt.Errorf("failed at path '%s', pattern contains unknown type", path)
	}
	return "", nil
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(log logr.Logger, resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string) (string, error) {
	patternMap = wildcards.ExpandInMetadata(patternMap, resourceMap)
	sortedResourceKeys := list.New()
	for k := range patternMap {
		sortedResourceKeys.PushBack(k)
	}

	for e := sortedResourceKeys.Front(); e != nil; e = e.Next() {
		key := e.Value.(string)
		handler := NewHandler(key, patternMap[key], path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern)
		if err != nil {
			return handlerPath, err
		}
	}
	return "", nil
}

// If validateResourceElement detects array element inside resource and pattern trees, it goes to validateArray
func validateArray(log logr.Logger, resourceArray, patternArray []interface{}, originPattern interface{}, path string) (string, error) {
	if 0 == len(patternArray) {
		return path, fmt.Errorf("Pattern Array empty")
	}

	switch patternArray[0].(type) {
	case map[string]interface{}:
		for _, patternElement := range patternArray {
			elemPath, err := validateArrayOfMaps(log, resourceArray, patternElement.(map[string]interface{}), originPattern, path)
			if err != nil {
				return elemPath, err
			}
		}
	default:
		if len(resourceArray) >= len(patternArray) {
			for i, patternElement := range patternArray {
				currentPath := path + strconv.Itoa(i) + "/"
				elemPath, err := validateResourceElement(log, resourceArray[i], patternElement, originPattern, currentPath)
				if err != nil {
					return elemPath, err
				}
			}
		} else {
			return "", fmt.Errorf("Validate Array failed, array length mismatch, resource Array len is %d and pattern Array len is %d", len(resourceArray), len(patternArray))
		}
	}
	return "", nil
}

// Matches all the elements in resource with the pattern
func validateArrayOfMaps(log logr.Logger, resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string) (string, error) {
	lengthOflenResourceMapArray := len(resourceMapArray) - 1
	for i, resourceElement := range resourceMapArray {
		currentPath := path + strconv.Itoa(i) + "/"
		returnpath, err := validateResourceElement(log, resourceElement, patternMap, originPattern, currentPath)
		if err != nil {
			if i < lengthOflenResourceMapArray {
				continue
			}
			return returnpath, err
		}
		break
	}
	return "", nil
}

func NewHandler(element string, pattern interface{}, path string) Handler {
	return Handler{
		element: element,
		pattern: pattern,
		path:    path,
	}
}

func (dh Handler) Handle(handler resourceElementHandler, resourceMap map[string]interface{}, originPattern interface{}) (string, error) {
	currentPath := dh.path + dh.element + "/"
	if dh.pattern == "*" && resourceMap[dh.element] != nil {
		return "", nil
	} else if dh.pattern == "*" && resourceMap[dh.element] == nil {
		return dh.path, fmt.Errorf("failed at path %s, field %s is not present", dh.path, dh.element)
	} else {
		path, err := handler(log.Log, resourceMap[dh.element], dh.pattern, originPattern, currentPath)
		if err != nil {
			return path, err
		}
	}
	return "", nil
}
