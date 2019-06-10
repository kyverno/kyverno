package engine

import (
	"encoding/json"
	"strconv"

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
	return validateResourceElement(resource, pattern, "/")
}

// validateResourceElement detects the element type (map, array, nil, string, int, bool, float)
// and calls corresponding handler
// Pattern tree and resource tree can have different structure. In this case validation fails
func validateResourceElement(resourceElement, patternElement interface{}, path string) result.RuleApplicationResult {
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

		return validateMap(typedResourceElement, typedPatternElement, path)
	// array
	case []interface{}:
		typedResourceElement, ok := resourceElement.([]interface{})
		if !ok {
			res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, patternElement, resourceElement)
			return res
		}

		return validateArray(typedResourceElement, typedPatternElement, path)
	// elementary values
	case string, float64, int, int64, bool, nil:
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
// For each element of the map we must detect the type again, so we pass this elements to validateResourceElement
func validateMap(resourceMap, patternMap map[string]interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")

	for key, patternElement := range patternMap {
		key = removeAnchor(key)

		// The '*' pattern means that key exists and has value
		if patternElement == "*" && resourceMap[key] != nil {
			continue
		} else if patternElement == "*" && resourceMap[key] == nil {
			res.FailWithMessagef("Field %s is not present", key)
		} else {
			elementResult := validateResourceElement(resourceMap[key], patternElement, path+key+"/")
			if result.Failed == elementResult.Reason {
				res.Reason = elementResult.Reason
				res.Messages = append(res.Messages, elementResult.Messages...)
			}
		}
	}

	return res
}

// If validateResourceElement detects array element inside resource and pattern trees, it goes to validateArray
// Unlike the validateMap, we should check the array elements type on-site, because in case of maps, we should
// get anchors and check each array element with it.
func validateArray(resourceArray, patternArray []interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")

	if 0 == len(patternArray) {
		return res
	}

	switch typedPatternElement := patternArray[0].(type) {
	case map[string]interface{}:
		// This is special case, because maps in arrays can have anchors that must be
		// processed with the special way affecting the entire array
		arrayResult := validateArrayOfMaps(resourceArray, typedPatternElement, path)
		res.MergeWith(&arrayResult)
	default:
		// In all other cases - detect type and handle each array element with validateResourceElement
		for i, patternElement := range patternArray {
			currentPath := path + strconv.Itoa(i) + "/"
			elementResult := validateResourceElement(resourceArray[i], patternElement, currentPath)
			res.MergeWith(&elementResult)
		}
	}

	return res
}

// validateArrayOfMaps gets anchors from pattern array map element, applies anchors logic
// and then validates each map due to the pattern
func validateArrayOfMaps(resourceMapArray []interface{}, patternMap map[string]interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")
	anchors := getAnchorsFromMap(patternMap)

	for i, resourceElement := range resourceMapArray {
		currentPath := path + strconv.Itoa(i) + "/"
		typedResourceElement, ok := resourceElement.(map[string]interface{})
		if !ok {
			res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, patternMap, resourceElement)
			return res
		}

		if skipArrayObject(typedResourceElement, anchors) {
			continue
		}

		mapValidationResult := validateMap(typedResourceElement, patternMap, currentPath)
		res.MergeWith(&mapValidationResult)
	}

	return res
}
