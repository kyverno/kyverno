package engine

import (
	"encoding/json"
	"fmt"
	"strconv"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	event "github.com/nirmata/kyverno/pkg/event"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate handles validating admission request
// Checks the target resourse for rules defined in the policy
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) event.KyvernoEvent {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	// Fill message at webhook server level
	policyEvent := &event.CompositeEvent{
		Message: fmt.Sprintf("policy - %s:", policy.Name),
		Reason:  event.PolicyApplied,
	}

	for _, rule := range policy.Spec.Rules {
		if rule.Validation == nil {
			continue
		}

		ruleEvent := event.RuleEvent{
			PolicyRule: rule.Name,
			Reason:     event.PolicyApplied,
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			ruleEvent.Messages = append(ruleEvent.Messages, fmt.Sprintf("Rule is not applicable to resource\n", rule.Name))
			policyEvent = event.Append(policyEvent, &ruleEvent)
			continue
		}

		ruleValidationEvent := validateResourceWithPattern(resource, rule.Validation.Pattern)
		if event.RequestBlocked == ruleValidationEvent.Reason {
			ruleEvent.Reason = ruleValidationEvent.Reason
			policyEvent.Reason = ruleValidationEvent.Reason
			ruleEvent.Messages = append(ruleEvent.Messages, ruleValidationEvent.Messages...)
			ruleEvent.Messages = append(ruleEvent.Messages, *rule.Validation.Message)
		} else {
			ruleEvent.Messages = append(ruleEvent.Messages, "Success")
		}

		policyEvent = event.Append(policyEvent, &ruleEvent)
	}

	return policyEvent
}

func validateResourceWithPattern(resource, pattern interface{}) event.RuleEvent {
	return validateResourceElement(resource, pattern, "/")
}

func validateResourceElement(value, pattern interface{}, path string) event.RuleEvent {
	result := event.RuleEvent{
		Reason:   event.PolicyApplied,
		Messages: []string{},
	}

	// TODO: Move similar message templates to message package

	switch typedPattern := pattern.(type) {
	case map[string]interface{}:
		typedValue, ok := value.(map[string]interface{})
		if !ok {
			result.Reason = event.RequestBlocked

			message := fmt.Sprintf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", pattern, value, path)
			result.Messages = append(result.Messages, message)
			return result
		}

		return validateMap(typedValue, typedPattern, path)
	case []interface{}:
		typedValue, ok := value.([]interface{})
		if !ok {
			result.Reason = event.RequestBlocked

			message := fmt.Sprintf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", pattern, value, path)
			result.Messages = append(result.Messages, message)
			return result
		}

		return validateArray(typedValue, typedPattern, path)
	case string, float64, int, int64, bool:
		if !ValidateValueWithPattern(value, pattern) {
			result.Reason = event.RequestBlocked

			message := fmt.Sprintf("Failed to validate value %v with pattern %v. Path: %s", value, pattern, path)
			result.Messages = append(result.Messages, message)
		}

		return result
	default:
		result.Reason = event.RequestBlocked

		message := fmt.Sprintf("Pattern contains unknown type %T. Path: %s", pattern, path)
		result.Messages = append(result.Messages, message)
		return result
	}
}

func validateMap(valueMap, patternMap map[string]interface{}, path string) event.RuleEvent {
	result := event.RuleEvent{
		Reason:   event.PolicyApplied,
		Messages: []string{},
	}

	for key, pattern := range patternMap {
		if wrappedWithParentheses(key) {
			key = key[1 : len(key)-1]
		}

		elementEvent := validateResourceElement(valueMap[key], pattern, path+key+"/")
		if event.RequestBlocked == elementEvent.Reason {
			result.Reason = elementEvent.Reason
			result.Messages = append(result.Messages, elementEvent.Messages...)
		}
	}

	return result
}

func validateArray(resourceArray, patternArray []interface{}, path string) event.RuleEvent {
	result := event.RuleEvent{
		Reason:   event.PolicyApplied,
		Messages: []string{},
	}

	if 0 == len(patternArray) {
		return result
	}

	switch pattern := patternArray[0].(type) {
	case map[string]interface{}:
		anchors := GetAnchorsFromMap(pattern)
		for i, value := range resourceArray {
			currentPath := path + strconv.Itoa(i) + "/"
			resource, ok := value.(map[string]interface{})
			if !ok {
				result.Reason = event.RequestBlocked

				message := fmt.Sprintf("Pattern and resource have different structures. Path: %s. Expected %T, found %T", pattern, value, currentPath)
				result.Messages = append(result.Messages, message)
				return result
			}

			if skipArrayObject(resource, anchors) {
				continue
			}

			mapEvent := validateMap(resource, pattern, currentPath)
			if event.RequestBlocked == mapEvent.Reason {
				result.Reason = mapEvent.Reason
				result.Messages = append(result.Messages, mapEvent.Messages...)
			}
		}
	case string, float64, int, int64, bool:
		for _, value := range resourceArray {
			if !ValidateValueWithPattern(value, pattern) {
				result.Reason = event.RequestBlocked

				message := fmt.Sprintf("Failed to validate value %v with pattern %v. Path: %s", value, pattern, path)
				result.Messages = append(result.Messages, message)
			}
		}
	default:
		result.Reason = event.RequestBlocked

		message := fmt.Sprintf("Array element pattern of unknown type %T. Path: %s", pattern, path)
		result.Messages = append(result.Messages, message)
	}

	return result
}
