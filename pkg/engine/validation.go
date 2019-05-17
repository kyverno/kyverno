package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/minio/minio/pkg/wildcard"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: This operators are already implemented in kubernetes
type Operator string

const (
	MoreEqual Operator = ">="
	LessEqual Operator = "<="
	NotEqual  Operator = "!="
	More      Operator = ">"
	Less      Operator = "<"
)

// TODO: Refactor using State pattern

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

		if !validateMap(resource, rule.Validation.Pattern) {
			log.Printf("Validation with the rule %s has failed: %s\n", rule.Name, *rule.Validation.Message)
			allowed = false
		} else {
			log.Printf("Validation rule %s is successful\n", rule.Name)
		}
	}

	return allowed
}

func validateMap(resourcePart, patternPart interface{}) bool {
	pattern, ok := patternPart.(map[string]interface{})

	if !ok {
		fmt.Printf("Validating error: expected Map, found %T\n", patternPart)
		return false
	}

	resource, ok := resourcePart.(map[string]interface{})

	if !ok {
		fmt.Printf("Validating error: expected Map, found %T\n", resourcePart)
		return false
	}

	for key, value := range pattern {
		if wrappedWithParentheses(key) {
			key = key[1 : len(key)-1]
		}

		if !validateMapElement(resource[key], value) {
			return false
		}
	}

	return true
}

func validateArray(resourcePart, patternPart interface{}) bool {
	patternArray, ok := patternPart.([]interface{})

	if !ok {
		fmt.Printf("Validating error: expected array, found %T\n", patternPart)
		return false
	}

	resourceArray, ok := resourcePart.([]interface{})

	if !ok {
		fmt.Printf("Validating error: expected array, found %T\n", resourcePart)
		return false
	}

	switch pattern := patternArray[0].(type) {
	case map[string]interface{}:
		anchors, err := getAnchorsFromMap(pattern)
		if err != nil {
			fmt.Printf("Validating error: %v\n", err)
			return false
		}

		for _, value := range resourceArray {
			resource, ok := value.(map[string]interface{})
			if !ok {
				fmt.Printf("Validating error: expected Map, found %T\n", resourcePart)
				return false
			}

			if skipArrayObject(resource, anchors) {
				continue
			}

			if !validateMap(resource, pattern) {
				return false
			}
		}

		return true
	default:
		for _, value := range resourceArray {
			if !checkSingleValue(value, patternArray[0]) {
				return false
			}
		}
	}

	return true
}

func validateMapElement(resourcePart, patternPart interface{}) bool {
	switch pattern := patternPart.(type) {
	case map[string]interface{}:
		dictionary, ok := resourcePart.(map[string]interface{})

		if !ok {
			fmt.Printf("Validating error: expected %T, found %T\n", patternPart, resourcePart)
			return false
		}

		return validateMap(dictionary, pattern)
	case []interface{}:
		array, ok := resourcePart.([]interface{})

		if !ok {
			fmt.Printf("Validating error: expected %T, found %T\n", patternPart, resourcePart)
			return false
		}

		return validateArray(array, pattern)
	case string:
		str, ok := resourcePart.(string)

		if !ok {
			fmt.Printf("Validating error: expected %T, found %T\n", patternPart, resourcePart)
			return false
		}

		return checkSingleValue(str, pattern)
	default:
		fmt.Printf("Validating error: unknown type in map: %T\n", patternPart)
		return false
	}
}

func getAnchorsFromMap(pattern map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range pattern {
		if wrappedWithParentheses(key) {
			result[key] = value
		}
	}

	return result, nil
}

func skipArrayObject(object, anchors map[string]interface{}) bool {
	for key, pattern := range anchors {
		key = key[1 : len(key)-1]

		value, ok := object[key]
		if !ok {
			return true
		}

		if !checkSingleValue(value, pattern) {
			return true
		}
	}

	return false
}

func checkSingleValue(value, pattern interface{}) bool {
	switch typedPattern := pattern.(type) {
	case string:
		switch typedValue := value.(type) {
		case string:
			return checkForWildcard(typedValue, typedPattern)
		case float64:
			return checkForOperator(typedValue, typedPattern)
		case int:
			return checkForOperator(float64(typedValue), typedPattern)
		default:
			fmt.Printf("Validating error: expected string or numerical type, found %T, pattern: %s\n", value, typedPattern)
			return false
		}
	case float64:
		num, ok := value.(float64)
		if !ok {
			fmt.Printf("Validating error: expected float, found %T\n", value)
			return false
		}

		return typedPattern == num
	case int:
		num, ok := value.(int)
		if !ok {
			fmt.Printf("Validating error: expected int, found %T\n", value)
			return false
		}

		return typedPattern == num
	default:
		fmt.Printf("Validating error: expected pattern (string or numerical type), found %T\n", pattern)
		return false
	}
}

func checkForWildcard(value, pattern string) bool {
	return wildcard.Match(pattern, value)
}

func checkForOperator(value float64, pattern string) bool {
	operators := strings.Split(pattern, "|")

	for _, operator := range operators {
		operator = strings.Replace(operator, " ", "", -1)
		if checkSingleOperator(value, operator) {
			return true
		}
	}

	return false
}

func checkSingleOperator(value float64, pattern string) bool {
	if operatorVal, err := strconv.ParseFloat(pattern, 64); err == nil {
		return value == operatorVal
	}

	if len(pattern) < 2 {
		fmt.Printf("Validating error: operator can't have less than 2 characters: %s\n", pattern)
		return false
	}

	if operatorVal, ok := parseOperator(MoreEqual, pattern); ok {
		return value >= operatorVal
	}

	if operatorVal, ok := parseOperator(LessEqual, pattern); ok {
		return value <= operatorVal
	}

	if operatorVal, ok := parseOperator(More, pattern); ok {
		return value > operatorVal
	}

	if operatorVal, ok := parseOperator(Less, pattern); ok {
		return value < operatorVal
	}

	if operatorVal, ok := parseOperator(NotEqual, pattern); ok {
		return value != operatorVal
	}

	fmt.Printf("Validating error: unknown operator: %s\n", pattern)
	return false
}

func parseOperator(operator Operator, pattern string) (float64, bool) {
	if pattern[:len(operator)] == string(operator) {
		if value, err := strconv.ParseFloat(pattern[len(operator):len(pattern)], 64); err == nil {
			return value, true
		}
	}

	return 0.0, false
}

func wrappedWithParentheses(str string) bool {
	if len(str) < 2 {
		return false
	}

	return (str[0] == '(' && str[len(str)-1] == ')')
}
