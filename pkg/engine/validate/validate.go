package validate

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/anchor"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/operator"
	"github.com/nirmata/kyverno/pkg/engine/variables"
)

// validateResourceWithPattern is a start of element-by-element validation process
// It assumes that validation is started from root, so "/" is passed
//TODO: for failure, we return the path at which it failed along with error
func ValidateResourceWithPattern(ctx context.EvalInterface, resource, pattern interface{}) (string, error) {
	// first pass we substitute all the JMESPATH substitution for the variable
	// variable: {{<JMESPATH>}}
	// if a JMESPATH fails, we dont return error but variable is substitured with nil and error log
	pattern = variables.SubstituteVariables(ctx, pattern)
	return validateResourceElement(resource, pattern, pattern, "/")
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(resourceElement, patternElement, originPattern interface{}, path string) (string, error) {
	var err error
	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			glog.V(4).Infof("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return path, fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}

		return validateMap(typedResourceElement, typedPatternElement, originPattern, path)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			glog.V(4).Infof("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return path, fmt.Errorf("Validation rule Failed at path %s, resource does not satisfy the expected overlay pattern", path)
		}

		return validateArray(typedResourceElement, typedPatternElement, originPattern, path)
	// elementary values
	case string, float64, int, int64, bool, nil:
		/*Analyze pattern */
		if checkedPattern := reflect.ValueOf(patternElement); checkedPattern.Kind() == reflect.String {
			if isStringIsReference(checkedPattern.String()) { //check for $ anchor
				patternElement, err = actualizePattern(originPattern, checkedPattern.String(), path)
				if err != nil {
					return path, err
				}
			}
		}
		if !ValidateValueWithPattern(resourceElement, patternElement) {
			return path, fmt.Errorf("Validation rule failed at '%s' to validate value %v with pattern %v", path, resourceElement, patternElement)
		}

	default:
		glog.V(4).Infof("Pattern contains unknown type %T. Path: %s", patternElement, path)
		return path, fmt.Errorf("Validation rule failed at '%s', pattern contains unknown type", path)
	}
	return "", nil
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string) (string, error) {
	// check if there is anchor in pattern
	// Phase 1 : Evaluate all the anchors
	// Phase 2 : Evaluate non-anchors
	anchors, resources := anchor.GetAnchorsResourcesFromMap(patternMap)

	// Evaluate anchors
	for key, patternElement := range anchors {
		// get handler for each pattern in the pattern
		// - Conditional
		// - Existance
		// - Equality
		handler := anchor.CreateElementHandler(key, patternElement, path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern)
		// if there are resource values at same level, then anchor acts as conditional instead of a strict check
		// but if there are non then its a if then check
		if err != nil {
			// If Conditional anchor fails then we dont process the resources
			if anchor.IsConditionAnchor(key) {
				glog.V(4).Infof("condition anchor did not satisfy, wont process the resources: %s", err)
				return "", nil
			}
			return handlerPath, err
		}
	}
	// Evaluate resources
	for key, resourceElement := range resources {
		// get handler for resources in the pattern
		handler := anchor.CreateElementHandler(key, resourceElement, path)
		handlerPath, err := handler.Handle(validateResourceElement, resourceMap, origPattern)
		if err != nil {
			return handlerPath, err
		}
	}
	return "", nil
}

func validateArray(resourceArray, patternArray []interface{}, originPattern interface{}, path string) (string, error) {

	if 0 == len(patternArray) {
		return path, fmt.Errorf("Pattern Array empty")
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		path, err := validateArrayOfMaps(resourceArray, typedPatternElement, originPattern, path)
		if err != nil {
			return path, err
		}
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		for i, patternElement := range patternArray {
			currentPath := path + strconv.Itoa(i) + "/"
			path, err := validateResourceElement(resourceArray[i], patternElement, originPattern, currentPath)
			if err != nil {
				return path, err
			}
		}
	}

	return "", nil
}

func actualizePattern(origPattern interface{}, referencePattern, absolutePath string) (interface{}, error) {
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

	valFromReference, err := getValueFromReference(origPattern, actualPath)
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
	if filepath.IsAbs(referencePath) {
		return referencePath
	}

	return filepath.Join(absolutePath, referencePath)
}

//Prepares original pattern, path to value, and call traverse function
func getValueFromReference(origPattern interface{}, reference string) (interface{}, error) {
	originalPatternMap := origPattern.(map[string]interface{})
	reference = reference[1:len(reference)]
	statements := strings.Split(reference, "/")

	return getValueFromPattern(originalPatternMap, statements, 0)
}

func getValueFromPattern(patternMap map[string]interface{}, keys []string, currentKeyIndex int) (interface{}, error) {

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
						glog.V(4).Infof("Pattern and resource have different structures. Expected %T, found %T", pattern, value)
						return nil, fmt.Errorf("Validation rule failed, resource does not have expected pattern %v", patternMap)
					}
					if keys[currentKeyIndex+1] == strconv.Itoa(i) {
						return getValueFromPattern(resourceMap, keys, currentKeyIndex+2)
					}
					return nil, errors.New("Reference to non-existent place in the document")
				}
			}
			return nil, errors.New("Reference to non-existent place in the document")
		case map[string]interface{}:
			if keys[currentKeyIndex] == rawKey {
				return getValueFromPattern(typedPattern, keys, currentKeyIndex+1)
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
func validateArrayOfMaps(resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string) (string, error) {
	for i, resourceElement := range resourceMapArray {
		// check the types of resource element
		// expect it to be map, but can be anything ?:(
		currentPath := path + strconv.Itoa(i) + "/"
		returnpath, err := validateResourceElement(resourceElement, patternMap, originPattern, currentPath)
		if err != nil {
			return returnpath, err
		}
	}
	return "", nil
}
