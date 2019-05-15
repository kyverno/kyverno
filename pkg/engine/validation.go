package engine

import (
	"encoding/json"
	"fmt"
	"log"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Validate handles validating admission request
// Checks the target resourse for rules defined in the policy
func Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) bool {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	allowed := true
	for i, rule := range policy.Spec.Rules {

		// Checks for preconditions
		// TODO: Rework PolicyEngine interface that it receives not a policy, but mutation object for
		// Mutate, validation for Validate and so on. It will allow to bring this checks outside of PolicyEngine
		// to common part as far as they present for all: mutation, validation, generation

		err := rule.Validate()
		if err != nil {
			log.Printf("Rule has invalid structure: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		ok, err := ResourceMeetsRules(rawResource, rule.ResourceDescription, gvk)
		if err != nil {
			log.Printf("Rule has invalid data: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if !ok {
			log.Printf("Rule is not applicable to the request: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if rule.Validation == nil {
			continue
		}

		if err := traverseAndValidate(resource, rule.Validation.Pattern); err != nil {
			log.Printf("Validation with the rule %s has failed %s: %s\n", rule.Name, err.Error(), *rule.Validation.Message)
			allowed = false
		} else {
			log.Printf("Validation rule %s is successful %s: %s\n", rule.Name, err.Error(), *rule.Validation.Message)
		}
	}

	return allowed
}

func validateMap(resourcePart, patternPart interface{}) error {
	pattern := patternPart.(map[string]interface{})
	resource, ok := resourcePart.(map[string]interface{})

	if !ok {
		return fmt.Errorf("Validating error: expected Map, found %T", resourcePart)
	}

	for key, value := range pattern {
		err := validateMapElement(resource[key], value)

		if err != nil {
			return err
		}
	}

	return nil
}

func validateArray(resourcePart, patternPart interface{}) error {
	pattern := patternPart.([]interface{})
	resource, ok := resourcePart.([]interface{})

	if !ok {
		return fmt.Errorf("Validating error: expected Map, found %T", resourcePart)
	}

	patternElem := pattern[0]
	switch typedElem := patternElem.(type) {
	case map[string]interface{}:
        
	default:
		return nil

	return nil
}

func validateMapElement(resourcePart, patternPart interface{}) error {
	switch pattern := patternPart.(type) {
	case map[string]interface{}:
		dictionary, ok := resourcePart.(map[string]interface{})

		if !ok {
			return fmt.Errorf("Validating error: expected %T, found %T", patternPart, resourcePart)
		}

		return validateMap(dictionary, pattern)
	case []interface{}:
		array, ok := resourcePart.([]interface{})

		if !ok {
			return fmt.Errorf("Validating error: expected %T, found %T", patternPart, resourcePart)
		}

		return validateArray(array, pattern)
	case string:
		str, ok := resourcePart.(string)

		if !ok {
			return fmt.Errorf("Validating error: expected %T, found %T", patternPart, resourcePart)
		}

		return validateSingleString(str, pattern)
	default:
		return fmt.Errorf("Received unknown type: %T", patternPart)
	}

	return nil
}

func validateSingleString(str, pattern string) error {
	if wrappedWithParentheses(str) {

	}

	return nil
}

func wrappedWithParentheses(str string) bool {
	return (str[0] == '(' && str[len(str)-1] == ')')
}

func checkForWildcard(value, pattern string) bool {
	return value == pattern
}

func checkForOperator(value int, pattern string) bool {
	return true
}

func getAnchorsFromMap(patternMap map[string]interface{}) map[string]string {
	result := make(map[string]Vertex)

	for key, value := range patternMap {
		str, ok := value.(string)
		if ok {
            result[key] = str
		}
	}
}
