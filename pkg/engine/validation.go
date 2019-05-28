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
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) event.Event {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	// Fill policy and target resource at webhook server level
	validationEvent := event.Event{
		Reason:   event.PolicyApplied,
		Messages: []string{},
	}

	for _, rule := range policy.Spec.Rules {
		if rule.Validation == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			message := fmt.Sprintf("Rule \"%s\" is not applicable to resource\n", rule.Name)
			validationEvent.Messages = append(validationEvent.Messages, message)
			continue
		}

		ruleEvent := validateResourceWithPattern(resource, rule.Validation.Pattern)
		if event.RequestBlocked == ruleEvent.Reason {
			validationEvent.Reason = ruleEvent.Reason
			validationEvent.Messages = append(validationEvent.Messages, ruleEvent.Messages...)
			validationEvent.Messages = append(validationEvent.Messages, *rule.Validation.Message)
		}
	}

	return validationEvent
}

func validateResourceWithPattern(resource, pattern interface{}) event.Event {
	return validateResourceElement(resource, pattern, "/")
}

func validateResourceElement(value, pattern interface{}, path string) event.Event {
	result := event.Event{
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

func validateMap(valueMap, patternMap map[string]interface{}, path string) event.Event {
	result := event.Event{
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

func validateArray(resourceArray, patternArray []interface{}, path string) event.Event {
	result := event.Event{
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

			if skipValidatingObject(resource, anchors) {
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

func skipValidatingObject(object, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := object[key]
		if !ok {
			return true
		}

		if !ValidateValueWithPattern(value, pattern) {
			return true
		}
	}

	return false
}
