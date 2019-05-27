package engine

import (
	"encoding/json"
	"fmt"
	"log"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Refactor using State pattern
// TODO: Return Events and pass all checks to get all validation errors (not )

// Validate handles validating admission request
// Checks the target resourse for rules defined in the policy
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) error {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	for _, rule := range policy.Spec.Rules {
		if rule.Validation == nil {
			continue
		}

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			log.Printf("Rule \"%s\" is not applicable to resource\n", rule.Name)
			continue
		}

		if err := validateMap(resource, rule.Validation.Pattern); err != nil {
			message := *rule.Validation.Message
			if len(message) == 0 {
				message = fmt.Sprintf("%v", err)
			} else {
				message = fmt.Sprintf("%s, %s", message, err.Error())
			}

			return fmt.Errorf("%s: %s", *rule.Validation.Message, err.Error())
		}
	}

	return nil
}

func validateMap(resourcePart, patternPart interface{}) error {
	pattern, ok := patternPart.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map, found %T", patternPart)
	}

	resource, ok := resourcePart.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map, found %T", resourcePart)
	}

	for key, value := range pattern {
		if wrappedWithParentheses(key) {
			key = key[1 : len(key)-1]
		}

		if err := validateMapElement(resource[key], value); err != nil {
			return err
		}
	}

	return nil
}

func validateArray(resourcePart, patternPart interface{}) error {
	patternArray, ok := patternPart.([]interface{})
	if !ok {
		return fmt.Errorf("expected array, found %T", patternPart)
	}

	resourceArray, ok := resourcePart.([]interface{})
	if !ok {
		return fmt.Errorf("expected array, found %T", resourcePart)
	}

	switch pattern := patternArray[0].(type) {
	case map[string]interface{}:
		anchors := GetAnchorsFromMap(pattern)

		for _, value := range resourceArray {
			resource, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("expected array, found %T", resourcePart)
			}

			if skipValidatingObject(resource, anchors) {
				continue
			}

			if err := validateMap(resource, pattern); err != nil {
				return err
			}
		}
	default:
		for _, value := range resourceArray {
			if !ValidateValueWithPattern(value, patternArray[0]) {
				return fmt.Errorf("Failed validate %v wit %v", value, patternArray[0])
			}
		}
	}

	return nil
}

func validateMapElement(resourcePart, patternPart interface{}) error {
	switch pattern := patternPart.(type) {
	case map[string]interface{}:
		dictionary, ok := resourcePart.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected %T, found %T", patternPart, resourcePart)
		}

		return validateMap(dictionary, pattern)
	case []interface{}:
		array, ok := resourcePart.([]interface{})
		if !ok {
			return fmt.Errorf("expected %T, found %T", patternPart, resourcePart)
		}

		return validateArray(array, pattern)
	case string, float64, int, int64, bool:
		if !ValidateValueWithPattern(resourcePart, patternPart) {
			return fmt.Errorf("Failed validate %v wit %v", resourcePart, patternPart)
		}
	default:
		return fmt.Errorf("validating error: unknown type in map: %T", patternPart)
	}

	return nil
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

func wrappedWithParentheses(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}
