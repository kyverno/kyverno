package engine

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate handles validating admission request
// Checks the target resources for rules defined in the policy
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) result.Result {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	policyResult := result.NewPolicyApplicationResult(policy.Name)

	for _, rule := range policy.Spec.Rules {
		if rule.Validation == nil {
			continue
		}

		ruleApplicationResult := result.NewRuleApplicationResult(rule.Name)

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			ruleApplicationResult.AddMessagef("Rule %s is not applicable to resource\n", rule.Name)
			policyResult = result.Append(policyResult, &ruleApplicationResult)
			continue
		}

		validationResult := validateResourceWithPattern(resource, rule.Validation.Pattern)
		if result.Success != validationResult.Reason {
			ruleApplicationResult.AddMessagef(*rule.Validation.Message)
			ruleApplicationResult.MergeWith(&validationResult)
		} else {
			ruleApplicationResult.AddMessagef("Success")
		}

		policyResult = result.Append(policyResult, &ruleApplicationResult)
	}

	return policyResult
}

// validateResourceWithPattern is a start of element-by-element validation process
// It assumes that validation is started from root, so "/" is passed
func validateResourceWithPattern(resource, pattern interface{}) result.RuleApplicationResult {
	return validateResourceElement(resource, pattern, pattern, "/")
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(resourceElement, patternElement, originPattern interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")
	// TODO: Move similar message templates to message package

	switch typedPatternElement := patternElement.(type) {
	// map
	case map[string]interface{}:
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return res
		}

		return validateMap(typedResourceElement, typedPatternElement, originPattern, path)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return res
		}

		return validateArray(typedResourceElement, typedPatternElement, originPattern, path)
	// elementary values
	case string, float64, int, int64, bool, nil:
		/*Analyze pattern */
		if checkedPattern := reflect.ValueOf(patternElement); checkedPattern.Kind() == reflect.String {
			if isStringIsReference(checkedPattern.String()) { //check for $ anchor
				patternElement, res = actualizePattern(originPattern, checkedPattern.String(), path)
				if result.Failed == res.Reason {
					return res
				}
			}
		}
		if !ValidateValueWithPattern(resourceElement, patternElement) {
			res.FailWithMessagef("Failed to validate value %v with pattern %v. Path: %s", resourceElement, patternElement, path)
		}

		return res
	default:
		res.FailWithMessagef("Pattern contains unknown type %T. Path: %s", patternElement, path)
		return res
	}
}

// If validateResourceElement detects map element inside resource and pattern trees, it goes to validateMap
// For each element of the map we must detect the type again, so we pass these elements to validateResourceElement
func validateMap(resourceMap, patternMap map[string]interface{}, origPattern interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")

	for key, patternElement := range patternMap {
		key = removeAnchor(key)

		// The '*' pattern means that key exists and has value
		if patternElement == "*" && resourceMap[key] != nil {
			continue
		} else if patternElement == "*" && resourceMap[key] == nil {
			res.FailWithMessagef("Field %s is not present", key)
		} else {
			elementResult := validateResourceElement(resourceMap[key], patternElement, origPattern, path+key+"/")
			res.MergeWith(&elementResult)
		}
	}

	return res
}

func validateArray(resourceArray, patternArray []interface{}, originPattern interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")

	if 0 == len(patternArray) {
		return res
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		arrayResult := validateArrayOfMaps(resourceArray, typedPatternElement, originPattern, path)
		res.MergeWith(&arrayResult)
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		for i, patternElement := range patternArray {
			currentPath := path + strconv.Itoa(i) + "/"
			elementResult := validateResourceElement(resourceArray[i], patternElement, originPattern, currentPath)
			res.MergeWith(&elementResult)
		}
	}

	return res
}

func actualizePattern(origPattern interface{}, referencePattern, absolutePath string) (interface{}, result.RuleApplicationResult) {
	res := result.NewRuleApplicationResult("")
	var foundValue interface{}

	referencePattern = strings.Trim(referencePattern, "$()")

	operator := getOperatorFromStringPattern(referencePattern)
	referencePattern = referencePattern[len(operator):]

	if len(referencePattern) == 0 {
		res.FailWithMessagef("Expected path. Found empty reference")
		return nil, res
	}

	actualPath := FormAbsolutePath(referencePattern, absolutePath)

	valFromReference, res := getValueFromReference(origPattern, actualPath)

	if result.Failed == res.Reason {
		return nil, res
	}

	if operator == Equal { //if operator does not exist return raw value
		return valFromReference, res
	}

	foundValue, res = valFromReferenceToString(valFromReference, string(operator))

	return string(operator) + foundValue.(string), res
}

//Parse value to string
func valFromReferenceToString(value interface{}, operator string) (string, result.RuleApplicationResult) {
	res := result.NewRuleApplicationResult("")

	switch typed := value.(type) {
	case string:
		return typed, res
	case int, int64:
		return fmt.Sprintf("%d", value), res
	case float64:
		return fmt.Sprintf("%f", value), res
	default:
		res.FailWithMessagef("Incorrect expression. Operator %s does not match with value: %v", operator, value)
		return "", res
	}
}

func FormAbsolutePath(referencePath, absolutePath string) string {
	if filepath.IsAbs(referencePath) {
		return referencePath
	}

	return filepath.Join(absolutePath, referencePath)
}

//Prepares original pattern, path to value, and call traverse function
func getValueFromReference(origPattern interface{}, reference string) (interface{}, result.RuleApplicationResult) {
	originalPatternMap := origPattern.(map[string]interface{})
	reference = reference[1:len(reference)]
	statements := strings.Split(reference, "/")

	return getValueFromPattern(originalPatternMap, statements, 0)
}

func getValueFromPattern(patternMap map[string]interface{}, keys []string, currentKeyIndex int) (interface{}, result.RuleApplicationResult) {
	res := result.NewRuleApplicationResult("")

	for key, pattern := range patternMap {
		rawKey := getRawKeyIfWrappedWithAttributes(key)

		if rawKey == keys[len(keys)-1] && currentKeyIndex == len(keys)-1 {
			return pattern, res
		} else if rawKey != keys[currentKeyIndex] && currentKeyIndex != len(keys)-1 {
			continue
		}

		switch typedPattern := pattern.(type) {
		case []interface{}:
			if keys[currentKeyIndex] == rawKey {
				for i, value := range typedPattern {
					resourceMap, ok := value.(map[string]interface{})
					if !ok {
						res.FailWithMessagef("Pattern and resource have different structures. Expected %T, found %T", pattern, value)
						return nil, res
					}
					if keys[currentKeyIndex+1] == strconv.Itoa(i) {
						return getValueFromPattern(resourceMap, keys, currentKeyIndex+2)
					}
					res.FailWithMessagef("Reference to non-existent place in the document")
				}
			}
			res.FailWithMessagef("Reference to non-existent place in the document")
		case map[string]interface{}:
			if keys[currentKeyIndex] == rawKey {
				return getValueFromPattern(typedPattern, keys, currentKeyIndex+1)
			}
			res.FailWithMessagef("Reference to non-existent place in the document")
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
	res.FailWithMessagef("No value found for specified reference: %s", path)
	return nil, res
}

// validateArrayOfMaps gets anchors from pattern array map element, applies anchors logic
// and then validates each map due to the pattern
func validateArrayOfMaps(resourceMapArray []interface{}, patternMap map[string]interface{}, originPattern interface{}, path string) result.RuleApplicationResult {
	anchor, pattern := getAnchorFromMap(patternMap)

	handler := CreateAnchorHandler(anchor, pattern, path)
	return handler.Handle(resourceMapArray, patternMap, originPattern)
}
