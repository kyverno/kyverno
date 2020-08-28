package validate

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
	commonAnchors "github.com/nirmata/kyverno/pkg/engine/anchor/common"
	"github.com/nirmata/kyverno/pkg/engine/common"
	"github.com/nirmata/kyverno/pkg/engine/operator"
)

// ValidateResourceWithPattern is a start of element-by-element validation process
// It assumes that validation is started from root, so "/" is passed
func ValidateResourceWithPattern(log logr.Logger, resource, pattern interface{}) (string, error) {
	// newAnchorMap - to check anchor key has values
	ac := common.NewAnchorMap()
	path, err := validateResourceElement(log, resource, pattern, pattern, "/", ac)
	if err != nil {
		if !ac.IsAnchorError() {
			return path, err
		}
	}

	return "", nil
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(log logr.Logger, resourceElement, patternElement, originPattern interface{}, path string, ac *common.AnchorKey) (string, error) {
	var err error
	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			log.V(4).Info("Pattern and resource have different structures.", "path", path, "expected", fmt.Sprintf("%T", patternElement), "current", fmt.Sprintf("%T", resourceElement))
			return path, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}
		// CheckAnchorInResource - check anchor anchor key exists in resource and update the AnchorKey fields.
		ac.CheckAnchorInResource(typedPatternElement, typedResourceElement)
		return validateMap(log, typedResourceElement, typedPatternElement, originPattern, path, ac)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			log.V(4).Info("Pattern and resource have different structures.", "path", path, "expected", fmt.Sprintf("%T", patternElement), "current", fmt.Sprintf("%T", resourceElement))
			return path, fmt.Errorf("Validation rule Failed at path %s, resource does not satisfy the expected overlay pattern", path)
		}
		return validateArray(log, typedResourceElement, typedPatternElement, originPattern, path, ac)
	// elementary values
	case string, float64, int, int64, bool, nil:
		/*Analyze pattern */
		if checkedPattern := reflect.ValueOf(patternElement); checkedPattern.Kind() == reflect.String {
			if isStringIsReference(checkedPattern.String()) { //check for $ anchor
				patternElement, err = actualizePattern(log, originPattern, checkedPattern.String(), path)
				if err != nil {
					return path, err
				}
			}
		}
		if !ValidateValueWithPattern(log, resourceElement, patternElement) {
			return path, fmt.Errorf("Validation rule failed at '%s' to validate value '%v' with pattern '%v'", path, resourceElement, patternElement)
		}

	default:
		log.V(4).Info("Pattern contains unknown type", "path", path, "current", fmt.Sprintf("%T", patternElement))
		return path, fmt.Errorf("Validation rule failed at '%s', pattern contains unknown type", path)
	}
	return "", nil
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(log logr.Logger, resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string, ac *common.AnchorKey) (string, error) {
	// check if there is anchor in pattern
	// Phase 1 : Evaluate all the anchors
	// Phase 2 : Evaluate non-anchors
	anchors, resources := anchor.GetAnchorsResourcesFromMap(patternMap)

	// Evaluate anchors
	for key, patternElement := range anchors {
		// get handler for each pattern in the pattern
		// - Conditional
		// - Existence
		// - Equality
		handler := anchor.CreateElementHandler(key, patternElement, path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern, ac)
		// if there are resource values at same level, then anchor acts as conditional instead of a strict check
		// but if there are non then its a if then check
		if err != nil {
			// If Conditional anchor fails then we don't process the resources
			if commonAnchors.IsConditionAnchor(key) || commonAnchors.IsExistenceAnchor(key) {
				ac.AnchorError = err
				log.Error(err, "condition anchor did not satisfy, wont process the resource")
				return "", nil
			}
			return handlerPath, err
		}
	}
	// If anchor fails then succeed validate and skip further validation of recursion
	if ac.AnchorError != nil {
		return "", nil
	}

	// Evaluate resources
	// getSortedNestedAnchorResource - keeps the anchor key to start of the list
	sortedResourceKeys := getSortedNestedAnchorResource(resources)
	for e := sortedResourceKeys.Front(); e != nil; e = e.Next() {
		key := e.Value.(string)
		handler := anchor.CreateElementHandler(key, resources[key], path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern, ac)
		if err != nil {
			return handlerPath, err
		}
	}
	return "", nil
}

func validateArray(log logr.Logger, resourceArray, patternArray []interface{}, originPattern interface{}, path string, ac *common.AnchorKey) (string, error) {

	if 0 == len(patternArray) {
		return path, fmt.Errorf("Pattern Array empty")
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		path, err := validateArrayOfMaps(log, resourceArray, typedPatternElement, originPattern, path, ac)
		if err != nil {
			return path, err
		}
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		if len(resourceArray) >= len(patternArray) {
			for i, patternElement := range patternArray {
				currentPath := path + strconv.Itoa(i) + "/"
				path, err := validateResourceElement(log, resourceArray[i], patternElement, originPattern, currentPath, ac)
				if err != nil {
					return path, err
				}
			}
		} else {
			return "", fmt.Errorf("Validate Array failed, array length mismatch, resource Array len is %d and pattern Array len is %d", len(resourceArray), len(patternArray))
		}
	}
	return "", nil
}

func actualizePattern(log logr.Logger, origPattern interface{}, referencePattern, absolutePath string) (interface{}, error) {
	var foundValue interface{}

	referencePattern = strings.Trim(referencePattern, "$()")

	operatorVariable := operator.GetOperatorFromStringPattern(referencePattern)
	referencePattern = referencePattern[len(operatorVariable):]

	if len(referencePattern) == 0 {
		return nil, errors.New("Expected path. Found empty reference")
	}
	// Check for variables
	// substitute it from Context
	// remove abosolute path
	// {{ }}
	// value :=
	actualPath := formAbsolutePath(referencePattern, absolutePath)

	valFromReference, err := getValueFromReference(log, origPattern, actualPath)
	if err != nil {
		return err, nil
	}
	//TODO validate this
	if operatorVariable == operator.Equal { //if operator does not exist return raw value
		return valFromReference, nil
	}

	foundValue, err = valFromReferenceToString(valFromReference, string(operatorVariable))
	if err != nil {
		return "", err
	}
	return string(operatorVariable) + foundValue.(string), nil
}

//Parse value to string
func valFromReferenceToString(value interface{}, operator string) (string, error) {

	switch typed := value.(type) {
	case string:
		return typed, nil
	case int, int64:
		return fmt.Sprintf("%d", value), nil
	case float64:
		return fmt.Sprintf("%f", value), nil
	default:
		return "", fmt.Errorf("Incorrect expression. Operator %s does not match with value: %v", operator, value)
	}
}

// returns absolute path
func formAbsolutePath(referencePath, absolutePath string) string {
	if path.IsAbs(referencePath) {
		return referencePath
	}

	return path.Join(absolutePath, referencePath)
}

//Prepares original pattern, path to value, and call traverse function
func getValueFromReference(log logr.Logger, origPattern interface{}, reference string) (interface{}, error) {
	originalPatternMap := origPattern.(map[string]interface{})
	reference = reference[1:]
	statements := strings.Split(reference, "/")

	return getValueFromPattern(log, originalPatternMap, statements, 0)
}

func getValueFromPattern(log logr.Logger, patternMap map[string]interface{}, keys []string, currentKeyIndex int) (interface{}, error) {

	for key, pattern := range patternMap {
		rawKey := getRawKeyIfWrappedWithAttributes(key)

		if rawKey == keys[len(keys)-1] && currentKeyIndex == len(keys)-1 {
			return pattern, nil
		} else if rawKey != keys[currentKeyIndex] && currentKeyIndex != len(keys)-1 {
			continue
		}

		switch typedPattern := pattern.(type) {
		case []interface{}:
			if keys[currentKeyIndex] == rawKey {
				for i, value := range typedPattern {
					resourceMap, ok := value.(map[string]interface{})
					if !ok {
						log.V(4).Info("Pattern and resource have different structures.", "expected", fmt.Sprintf("%T", pattern), "current", fmt.Sprintf("%T", value))
						return nil, fmt.Errorf("Validation rule failed, resource does not have expected pattern %v", patternMap)
					}
					if keys[currentKeyIndex+1] == strconv.Itoa(i) {
						return getValueFromPattern(log, resourceMap, keys, currentKeyIndex+2)
					}
					// TODO : SA4004: the surrounding loop is unconditionally terminated (staticcheck)
					return nil, errors.New("Reference to non-existent place in the document")
				}
				return nil, nil // Just a hack to fix the lint
			}
			return nil, errors.New("Reference to non-existent place in the document")
		case map[string]interface{}:
			if keys[currentKeyIndex] == rawKey {
				return getValueFromPattern(log, typedPattern, keys, currentKeyIndex+1)
			}
			return nil, errors.New("Reference to non-existent place in the document")
		case string, float64, int, int64, bool, nil:
			continue
		}
	}

	path := ""

	for _, elem := range keys {
		path = "/" + elem + path
	}
	return nil, fmt.Errorf("No value found for specified reference: %s", path)
}

// validateArrayOfMaps gets anchors from pattern array map element, applies anchors logic
// and then validates each map due to the pattern
func validateArrayOfMaps(log logr.Logger, resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string, ac *common.AnchorKey) (string, error) {
	for i, resourceElement := range resourceMapArray {
		// check the types of resource element
		// expect it to be map, but can be anything ?:(
		currentPath := path + strconv.Itoa(i) + "/"
		returnpath, err := validateResourceElement(log, resourceElement, patternMap, originPattern, currentPath, ac)
		if err != nil {
			return returnpath, err
		}
	}
	return "", nil
}

func isStringIsReference(str string) bool {
	if len(str) < len(operator.ReferenceSign) {
		return false
	}

	return str[0] == '$' && str[1] == '(' && str[len(str)-1] == ')'
}
