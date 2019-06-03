package engine

import (
	"encoding/json"
	"fmt"
	"strconv"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate handles validating admission request
// Checks the target resourse for rules defined in the policy
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) result.Result {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	// Fill message at webhook server level
	policyResult := &result.CompositeResult{
		Message: fmt.Sprintf("policy - %s:", policy.Name),
		Reason:  result.PolicyApplied,
	}

	for _, rule := range policy.Spec.Rules {
		if rule.Validation == nil {
			continue
		}

		RuleApplicationResult := result.RuleApplicationResult{
			PolicyRule: rule.Name,
			Reason:     result.PolicyApplied,
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			RuleApplicationResult.Messages = append(RuleApplicationResult.Messages, fmt.Sprintf("Rule %s is not applicable to resource\n", rule.Name))
			policyResult = result.Append(policyResult, &RuleApplicationResult)
			continue
		}

		ruleValidationResult := validateResourceWithPattern(resource, rule.Validation.Pattern)
		if result.RequestBlocked == ruleValidationResult.Reason {
			RuleApplicationResult.Reason = ruleValidationResult.Reason
			policyResult.Reason = ruleValidationResult.Reason
			RuleApplicationResult.Messages = append(RuleApplicationResult.Messages, ruleValidationResult.Messages...)
			RuleApplicationResult.Messages = append(RuleApplicationResult.Messages, *rule.Validation.Message)
		} else {
			RuleApplicationResult.Messages = append(RuleApplicationResult.Messages, "Success")
		}

		policyResult = result.Append(policyResult, &RuleApplicationResult)
	}

	return policyResult
}

func validateResourceWithPattern(resource, pattern interface{}) result.RuleApplicationResult {
	return validateResourceElement(resource, pattern, "/")
}

func validateResourceElement(value, pattern interface{}, path string) result.RuleApplicationResult {
	res := result.RuleApplicationResult{
		Reason:   result.PolicyApplied,
		Messages: []string{},
	}

	// TODO: Move similar message templates to message package

	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		typedValue, ok := value.(map[string]interface{})
		if !ok {
			res.Reason = result.RequestBlocked

			message := fmt.Sprintf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", pattern, value, path)
			res.Messages = append(res.Messages, message)
			return res
		}

		return validateMap(typedValue, typedPattern, path)
	case []interface{}:
		typedValue, ok := value.([]interface{})
		if !ok {
			res.Reason = result.RequestBlocked

			message := fmt.Sprintf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", pattern, value, path)
			res.Messages = append(res.Messages, message)
			return res
		}

		return validateArray(typedValue, typedPattern, path)
	case string, float64, int, int64, bool:
		if !ValidateValueWithPattern(value, pattern) {
			res.Reason = result.RequestBlocked

			message := fmt.Sprintf("Failed to validate value %v with pattern %v. Path: %s", value, pattern, path)
			res.Messages = append(res.Messages, message)
		}

		return res
	default:
		res.Reason = result.RequestBlocked

		message := fmt.Sprintf("Pattern contains unknown type %T. Path: %s", pattern, path)
		res.Messages = append(res.Messages, message)
		return res
	}
}

func validateMap(valueMap, patternMap map[string]interface{}, path string) result.RuleApplicationResult {
	res := result.RuleApplicationResult{
		Reason:   result.PolicyApplied,
		Messages: []string{},
	}

	for key, pattern := range patternMap {
		if wrappedWithParentheses(key) {
			key = key[1 : len(key)-1]
		}

		elementResult := validateResourceElement(valueMap[key], pattern, path+key+"/")
		if result.RequestBlocked == elementResult.Reason {
			res.Reason = elementResult.Reason
			res.Messages = append(res.Messages, elementResult.Messages...)
		}
	}

	return res
}

func validateArray(resourceArray, patternArray []interface{}, path string) result.RuleApplicationResult {
	res := result.RuleApplicationResult{
		Reason:   result.PolicyApplied,
		Messages: []string{},
	}

	if 0 == len(patternArray) {
		return res
	}

	switch pattern := patternArray[0].(type) {
	case map[string]interface{}:
		anchors := GetAnchorsFromMap(pattern)
		for i, value := range resourceArray {
			currentPath := path + strconv.Itoa(i) + "/"
			resource, ok := value.(map[string]interface{})
			if !ok {
				res.Reason = result.RequestBlocked

				message := fmt.Sprintf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", pattern, value, currentPath)
				res.Messages = append(res.Messages, message)
				return res
			}

			if skipArrayObject(resource, anchors) {
				continue
			}

			mapValidationResult := validateMap(resource, pattern, currentPath)
			if result.RequestBlocked == mapValidationResult.Reason {
				res.Reason = mapValidationResult.Reason
				res.Messages = append(res.Messages, mapValidationResult.Messages...)
			}
		}
	case string, float64, int, int64, bool:
		for _, value := range resourceArray {
			if !ValidateValueWithPattern(value, pattern) {
				res.Reason = result.RequestBlocked

				message := fmt.Sprintf("Failed to validate value %v with pattern %v. Path: %s", value, pattern, path)
				res.Messages = append(res.Messages, message)
			}
		}
	default:
		res.Reason = result.RequestBlocked

		message := fmt.Sprintf("Array element pattern of unknown type %T. Path: %s", pattern, path)
		res.Messages = append(res.Messages, message)
	}

	return res
}
