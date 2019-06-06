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
			ruleApplicationResult.MergeWith(&validationResult)
			ruleApplicationResult.AddMessagef(*rule.Validation.Message)
		} else {
			ruleApplicationResult.AddMessagef("Success")
		}

		policyResult = result.Append(policyResult, &ruleApplicationResult)
	}

	return policyResult
}

func validateResourceWithPattern(resource, pattern interface{}) result.RuleApplicationResult {
	return validateResourceElement(resource, pattern, "/")
}

func validateResourceElement(value, pattern interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")
	// TODO: Move similar message templates to message package

	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		typedValue, ok := value.(map[string]interface{})
		if !ok {
			res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, pattern, value)
			return res
		}

		return validateMap(typedValue, typedPattern, path)
	case []interface{}:
		typedValue, ok := value.([]interface{})
		if !ok {
			res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", path, pattern, value)
			return res
		}

		return validateArray(typedValue, typedPattern, path)
	case string, float64, int, int64, bool, nil:
		if !ValidateValueWithPattern(value, pattern) {
			res.FailWithMessagef("Failed to validate value %v with pattern %v. Path: %s", value, pattern, path)
		}

		return res
	default:
		res.FailWithMessagef("Pattern contains unknown type %T. Path: %s", pattern, path)
		return res
	}
}

func validateMap(valueMap, patternMap map[string]interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")

	for key, pattern := range patternMap {
		if wrappedWithParentheses(key) {
			key = key[1 : len(key)-1]
		}

		if pattern == "*" && valueMap[key] != nil {
			continue
		} else if pattern == "*" && valueMap[key] == nil {
			res.FailWithMessagef("Field %s is not present", key)
		} else {
			elementResult := validateResourceElement(valueMap[key], pattern, path+key+"/")
			if result.Failed == elementResult.Reason {
				res.Reason = elementResult.Reason
				res.Messages = append(res.Messages, elementResult.Messages...)
			}
		}

	}

	return res
}

func validateArray(resourceArray, patternArray []interface{}, path string) result.RuleApplicationResult {
	res := result.NewRuleApplicationResult("")

	if 0 == len(patternArray) {
		return res
	}

	switch pattern := patternArray[0].(type) {
	case map[string]interface{}:
		anchors := getAnchorsFromMap(pattern)
		for i, value := range resourceArray {
			currentPath := path + strconv.Itoa(i) + "/"
			resource, ok := value.(map[string]interface{})
			if !ok {
				res.FailWithMessagef("Pattern and resource have different structures. Path: %s. Expected %T, found %T", currentPath, pattern, value)
				return res
			}

			if skipArrayObject(resource, anchors) {
				continue
			}

			mapValidationResult := validateMap(resource, pattern, currentPath)
			if result.Failed == mapValidationResult.Reason {
				res.Reason = mapValidationResult.Reason
				res.Messages = append(res.Messages, mapValidationResult.Messages...)
			}
		}
	case string, float64, int, int64, bool, nil:
		for _, value := range resourceArray {
			if !ValidateValueWithPattern(value, pattern) {
				res.FailWithMessagef("Failed to validate value %v with pattern %v. Path: %s", value, pattern, path)
			}
		}
	default:
		res.FailWithMessagef("Array element pattern of unknown type %T. Path: %s", pattern, path)
	}

	return res
}
