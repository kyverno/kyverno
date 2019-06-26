package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate handles validating admission request
// Checks the target resources for rules defined in the policy
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) ([]*info.RuleInfo, error) {
	var resource interface{}
	ris := []*info.RuleInfo{}

	err := json.Unmarshal(rawResource, &resource)
	if err != nil {
		return nil, err
	}

	for _, rule := range policy.Spec.Rules {
		if rule.Validation == nil {
			continue
		}
		ri := info.NewRuleInfo(rule.Name, info.Validation)

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			glog.V(3).Info("Not applicable on specified resource kind%s", gvk.Kind)
			continue
		}

		err := validateResourceWithPattern(resource, rule.Validation.Pattern)
		if err != nil {
			ri.Fail()
			ri.Addf("Validation has failed. err %s", err)
		} else {
			ri.Add("Validation succesfully")

		}
		ris = append(ris, ri)
	}

	return ris, nil
}

// validateResourceWithPattern is a start of element-by-element validation process
// It assumes that validation is started from root, so "/" is passed
func validateResourceWithPattern(resource, pattern interface{}) error {
	return validateResourceElement(resource, pattern, pattern, "/")
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(resourceElement, patternElement, originPattern interface{}, path string) error {
	var err error
	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			return fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}

		return validateMap(typedResourceElement, typedPatternElement, originPattern, path)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			return fmt.Errorf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
		}

		return validateArray(typedResourceElement, typedPatternElement, originPattern, path)
	// elementary values
	case string, float64, int, int64, bool, nil:
		/*Analyze pattern */
		if checkedPattern := reflect.ValueOf(patternElement); checkedPattern.Kind() == reflect.String {
			if isStringIsReference(checkedPattern.String()) { //check for $ anchor
				patternElement, err = actualizePattern(originPattern, checkedPattern.String(), path)
				if err != nil {
					return err
				}
			}
		}
		if !ValidateValueWithPattern(resourceElement, patternElement) {
			return fmt.Errorf("Failed to validate value %v with pattern %v. Path: %s", resourceElement, patternElement, path)
		}

	default:
		return fmt.Errorf("Pattern contains unknown type %T. Path: %s", patternElement, path)
	}
	return nil
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string) error {

	for key, patternElement := range patternMap {
		key = removeAnchor(key)

		// The '*' pattern means that key exists and has value
		if patternElement == "*" && resourceMap[key] != nil {
			continue
		} else if patternElement == "*" && resourceMap[key] == nil {
			return fmt.Errorf("Field %s is not present", key)
		} else {
			err := validateResourceElement(resourceMap[key], patternElement, origPattern, path+key+"/")
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func validateArray(resourceArray, patternArray []interface{}, originPattern interface{}, path string) error {

	if 0 == len(patternArray) {
		return fmt.Errorf("Pattern Array empty")
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		err := validateArrayOfMaps(resourceArray, typedPatternElement, originPattern, path)
		if err != nil {
			return err
		}
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		for i, patternElement := range patternArray {
			currentPath := path + strconv.Itoa(i) + "/"
			err := validateResourceElement(resourceArray[i], patternElement, originPattern, currentPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func actualizePattern(origPattern interface{}, referencePattern, absolutePath string) (interface{}, error) {
	var foundValue interface{}

	referencePattern = strings.Trim(referencePattern, "$()")

	operator := getOperatorFromStringPattern(referencePattern)
	referencePattern = referencePattern[len(operator):]

	if len(referencePattern) == 0 {
		return nil, errors.New("Expected path. Found empty reference")
	}

	actualPath := FormAbsolutePath(referencePattern, absolutePath)

	valFromReference, err := getValueFromReference(origPattern, actualPath)
	if err != nil {
		return err, nil
	}
	//TODO validate this
	if operator == Equal { //if operator does not exist return raw value
		return valFromReference, nil
	}

	foundValue, err = valFromReferenceToString(valFromReference, string(operator))
	if err != nil {
		return "", err
	}
	return string(operator) + foundValue.(string), nil
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

func FormAbsolutePath(referencePath, absolutePath string) string {
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
						return nil, fmt.Errorf("Pattern and resource have different structures. Expected %T, found %T", pattern, value)
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

	/*for i := len(keys) - 1; i >= 0; i-- {
		path = keys[i] + path + "/"
	}*/
	for _, elem := range keys {
		path = "/" + elem + path
	}
	return nil, fmt.Errorf("No value found for specified reference: %s", path)
}

// validateArrayOfMaps gets anchors from pattern array map element, applies anchors logic
// and then validates each map due to the pattern
func validateArrayOfMaps(resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string) error {
	anchor, pattern := getAnchorFromMap(patternMap)

	handler := CreateAnchorHandler(anchor, pattern, path)
	return handler.Handle(resourceMapArray, patternMap, originPattern)
}
