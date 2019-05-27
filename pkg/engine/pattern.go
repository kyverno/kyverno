package engine

import (
	"log"
	"math"
	"strings"
)

// ValidateValueWithPattern validates value with operators and wildcards
func ValidateValueWithPattern(value, pattern interface{}) bool {
	switch typedPattern := pattern.(type) {
	case bool:
		typedValue, ok := value.(bool)
		if !ok {
			log.Printf("Expected bool, found %T", value)
			return false
		}
		return typedPattern == typedValue
	case int:
		return validateValueWithIntPattern(value, typedPattern)
	case float64:
		return validateValueWithFloatPattern(value, typedPattern)
	case string:
		return validateValueWithStringPattern(value, typedPattern)
	case map[string]interface{}, []interface{}:
		log.Println("Maps and arrays as patterns are not supported")
		return false
	case nil:
		return validateValueWithNilPattern(value)
	default:
		log.Printf("Unknown type as pattern: %T\n", pattern)
		return false
	}
}

func validateValueWithIntPattern(value interface{}, pattern int) bool {
	switch typedValue := value.(type) {
	case int:
		return typedValue == pattern
	case float64:
		// check that float has no fraction
		if typedValue == math.Trunc(typedValue) {
			return int(typedValue) == pattern
		}

		log.Printf("Expected int, found float: %f\n", typedValue)
		return false
	default:
		log.Printf("Expected int, found: %T\n", value)
		return false
	}
}

func validateValueWithFloatPattern(value interface{}, pattern float64) bool {
	switch typedValue := value.(type) {
	case int:
		// check that float has no fraction
		if pattern == math.Trunc(pattern) {
			return int(pattern) == value
		}

		log.Printf("Expected float, found int: %d\n", typedValue)
		return false
	case float64:
		return typedValue == pattern
	default:
		log.Printf("Expected float, found: %T\n", value)
		return false
	}
}

func validateValueWithNilPattern(value interface{}) bool {
	switch typed := value.(type) {
	case float64:
		return typed == 0.0
	case int:
		return typed == 0
	case string:
		return typed == ""
	case bool:
		return typed == false
	case nil:
		return true
	case map[string]interface{}, []interface{}:
		log.Println("Maps and arrays could not be checked with nil pattern")
		return false
	default:
		log.Printf("Unknown type as value when checking for nil pattern: %T\n", value)
		return false
	}
}

func validateValueWithStringPattern(value interface{}, pattern string) bool {
	statements := strings.Split(pattern, "|")
	for statement := range statements {
		if checkSingleStatement(value, statement) {
			return true
		}
	}

	return false
}

func checkSingleStatement(value, pattern interface{}) bool {
	return true
}
