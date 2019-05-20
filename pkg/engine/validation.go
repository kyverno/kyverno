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

// Operator is string alias that represents selection operators enum
type Operator string

const (
	MoreEqual Operator = ">="
	LessEqual Operator = "<="
	NotEqual  Operator = "!="
	More      Operator = ">"
	Less      Operator = "<"
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
			return fmt.Errorf("%s: %s", *rule.Validation.Message, err.Error())
		}
	}

	log.Println("Validation is successful")
	return nil
}

func validateMap(resourcePart, patternPart interface{}) error {
	pattern, ok := patternPart.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Expected map, found %T", patternPart)
	}

	resource, ok := resourcePart.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Expected map, found %T", resourcePart)
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
		return fmt.Errorf("Expected array, found %T", patternPart)
	}

	resourceArray, ok := resourcePart.([]interface{})
	if !ok {
		return fmt.Errorf("Expected array, found %T", resourcePart)
	}

	switch pattern := patternArray[0].(type) {
	case map[string]interface{}:
		anchors, err := getAnchorsFromMap(pattern)
		if err != nil {
			return err
		}

		for _, value := range resourceArray {
			resource, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("Expected array, found %T", resourcePart)
			}

			if skipArrayObject(resource, anchors) {
				continue
			}

			if err := validateMap(resource, pattern); err != nil {
				return err
			}
		}
	default:
		for _, value := range resourceArray {
			if err := checkSingleValue(value, patternArray[0]); err != nil {
				return err
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
			return fmt.Errorf("Expected %T, found %T", patternPart, resourcePart)
		}

		return validateMap(dictionary, pattern)
	case []interface{}:
		array, ok := resourcePart.([]interface{})
		if !ok {
			return fmt.Errorf("Expected %T, found %T", patternPart, resourcePart)
		}

		return validateArray(array, pattern)
	case string:
		str, ok := resourcePart.(string)

		if !ok {
			return fmt.Errorf("Expected %T, found %T", patternPart, resourcePart)
		}

		return checkSingleValue(str, pattern)
	default:
		return fmt.Errorf("Validating error: unknown type in map: %T", patternPart)
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

		if err := checkSingleValue(value, pattern); err != nil {
			return true
		}
	}

	return false
}

func checkSingleValue(value, pattern interface{}) error {
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
			return fmt.Errorf("Expected string or numerical type, found %T, pattern: %s", value, typedPattern)
		}
	case float64:
		num, ok := value.(float64)
		if !ok {
			return fmt.Errorf("Expected float, found %T", value)
		}

		if typedPattern != num {
			return fmt.Errorf("Value %f is not equal to pattern %f", value, typedPattern)
		}
	case int:
		num, ok := value.(int)
		if !ok {
			return fmt.Errorf("Expected int, found %T", value)
		}

		if typedPattern != num {
			return fmt.Errorf("Value %d is not equal to pattern %d", num, typedPattern)
		}
	default:
		return fmt.Errorf("Expected pattern (string or numerical type), found %T", pattern)
	}

	return nil
}

func checkForWildcard(value, pattern string) error {
	if !wildcard.Match(pattern, value) {
		return fmt.Errorf("Wildcard check has failed. Pattern: \"%s\". Value: \"%s\"", pattern, value)
	}

	return nil
}

func checkForOperator(value float64, pattern string) error {
	operators := strings.Split(pattern, "|")

	for _, operator := range operators {
		operator = strings.Replace(operator, " ", "", -1)

		// At least one success - return nil
		if checkSingleOperator(value, operator) {
			return nil
		}
	}

	return fmt.Errorf("Operator check has failed. Pattern: \"%s\". Value: \"%f\"", pattern, value)
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
