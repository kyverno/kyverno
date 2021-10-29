package validate

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	"github.com/minio/pkg/wildcard"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
)

type quantity int

const (
	equal       quantity = 0
	lessThan    quantity = -1
	greaterThan quantity = 1
)

// ValidateValueWithPattern validates value with operators and wildcards
func ValidateValueWithPattern(log logr.Logger, value, pattern interface{}) bool {
	switch typedPattern := pattern.(type) {
	case bool:
		typedValue, ok := value.(bool)
		if !ok {
			log.V(4).Info("Expected type bool", "type", fmt.Sprintf("%T", value), "value", value)
			return false
		}
		return typedPattern == typedValue
	case int:
		return validateValueWithIntPattern(log, value, int64(typedPattern))
	case int64:
		return validateValueWithIntPattern(log, value, typedPattern)
	case float64:
		return validateValueWithFloatPattern(log, value, typedPattern)
	case string:
		return validateValueWithStringPatterns(log, value, typedPattern)
	case nil:
		return validateValueWithNilPattern(log, value)
	case map[string]interface{}:
		return validateValueWithMapPattern(log, value, typedPattern)
	case []interface{}:
		log.Info("arrays are not supported as patterns")
		return false
	default:
		log.Info("Unknown type", "type", fmt.Sprintf("%T", typedPattern), "value", typedPattern)
		return false
	}
}

func validateValueWithMapPattern(log logr.Logger, value interface{}, typedPattern map[string]interface{}) bool {
	// verify the type of the resource value is map[string]interface,
	// we only check for existence of object, not the equality of content and value
	_, ok := value.(map[string]interface{})
	if !ok {
		log.Info("Expected type map[string]interface{}", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
	return true
}

// Handler for int values during validation process
func validateValueWithIntPattern(log logr.Logger, value interface{}, pattern int64) bool {
	switch typedValue := value.(type) {
	case int:
		return int64(typedValue) == pattern
	case int64:
		return typedValue == pattern
	case float64:
		// check that float has no fraction
		if typedValue == math.Trunc(typedValue) {
			return int64(typedValue) == pattern
		}

		log.Info("Expected type int", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
		return false
	case string:
		// extract int64 from string
		int64Num, err := strconv.ParseInt(typedValue, 10, 64)
		if err != nil {
			log.Error(err, "Failed to parse int64 from string")
			return false
		}
		return int64Num == pattern
	default:
		log.Info("Expected type int", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

// Handler for float values during validation process
func validateValueWithFloatPattern(log logr.Logger, value interface{}, pattern float64) bool {
	switch typedValue := value.(type) {
	case int:
		// check that float has no fraction
		if pattern == math.Trunc(pattern) {
			return int(pattern) == value
		}
		log.Info("Expected type float", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
		return false
	case int64:
		// check that float has no fraction
		if pattern == math.Trunc(pattern) {
			return int64(pattern) == value
		}
		log.Info("Expected type float", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
		return false
	case float64:
		return typedValue == pattern
	case string:
		// extract float64 from string
		float64Num, err := strconv.ParseFloat(typedValue, 64)
		if err != nil {
			log.Error(err, "Failed to parse float64 from string")
			return false
		}
		return float64Num == pattern
	default:
		log.Info("Expected type float", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

// Handler for nil values during validation process
func validateValueWithNilPattern(log logr.Logger, value interface{}) bool {
	switch typed := value.(type) {
	case float64:
		return typed == 0.0
	case int:
		return typed == 0
	case int64:
		return typed == 0
	case string:
		return typed == ""
	case bool:
		return !typed
	case nil:
		return true
	case map[string]interface{}, []interface{}:
		log.Info("Maps and arrays could not be checked with nil pattern")
		return false
	default:
		log.Info("Unknown type as value when checking for nil pattern", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

// Handler for pattern values during validation process
func validateValueWithStringPatterns(log logr.Logger, value interface{}, pattern string) bool {
	conditions := strings.Split(pattern, "|")
	for _, condition := range conditions {
		condition = strings.Trim(condition, " ")
		if checkForAndConditionsAndValidate(log, value, condition) {
			return true
		}
	}

	return false
}

func checkForAndConditionsAndValidate(log logr.Logger, value interface{}, pattern string) bool {
	conditions := strings.Split(pattern, "&")
	for _, condition := range conditions {
		condition = strings.Trim(condition, " ")
		if !validateValueWithStringPattern(log, value, condition) {
			return false
		}
	}

	return true
}

// Handler for single pattern value during validation process
// Detects if pattern has a number
func validateValueWithStringPattern(log logr.Logger, value interface{}, pattern string) bool {
	operatorVariable := operator.GetOperatorFromStringPattern(pattern)

	// Upon encountering InRange operator split the string by `-` and basically
	// verify the result of (x >= leftEndpoint & x <= rightEndpoint)
	if operatorVariable == operator.InRange {
		endpoints := strings.Split(pattern, "-")
		leftEndpoint, rightEndpoint := endpoints[0], endpoints[1]

		gt := validateValueWithStringPattern(log, value, fmt.Sprintf(">=%s", leftEndpoint))
		if !gt {
			return false
		}
		pattern = fmt.Sprintf("<=%s", rightEndpoint)
		operatorVariable = operator.LessEqual
	}

	// Upon encountering NotInRange operator split the string by `!-` and basically
	// verify the result of (x < leftEndpoint | x > rightEndpoint)
	if operatorVariable == operator.NotInRange {
		endpoints := strings.Split(pattern, "!-")
		leftEndpoint, rightEndpoint := endpoints[0], endpoints[1]

		lt := validateValueWithStringPattern(log, value, fmt.Sprintf("<%s", leftEndpoint))
		if lt {
			return true
		}
		pattern = fmt.Sprintf(">%s", rightEndpoint)
		operatorVariable = operator.More
	}

	pattern = pattern[len(operatorVariable):]
	pattern = strings.TrimSpace(pattern)
	number, str := getNumberAndStringPartsFromPattern(pattern)

	if number == "" {
		return validateString(log, value, str, operatorVariable)
	}

	return validateNumberWithStr(log, value, pattern, operatorVariable)
}

// Handler for string values
func validateString(log logr.Logger, value interface{}, pattern string, operatorVariable operator.Operator) bool {
	if operator.NotEqual == operatorVariable || operator.Equal == operatorVariable {
		var strValue string
		var ok bool = false
		switch v := value.(type) {
		case float64:
			strValue = strconv.FormatFloat(v, 'E', -1, 64)
			ok = true
		case int:
			strValue = strconv.FormatInt(int64(v), 10)
			ok = true
		case int64:
			strValue = strconv.FormatInt(v, 10)
			ok = true
		case string:
			strValue = v
			ok = true
		case bool:
			strValue = strconv.FormatBool(v)
			ok = true
		case nil:
			ok = false
		}
		if !ok {
			log.V(4).Info("unexpected type", "got", value, "expect", pattern)
			return false
		}

		wildcardResult := wildcard.Match(pattern, strValue)

		if operator.NotEqual == operatorVariable {
			return !wildcardResult
		}

		return wildcardResult
	}
	log.Info("Operators >, >=, <, <= are not applicable to strings")
	return false
}

// validateNumberWithStr compares quantity if pattern type is quantity
//  or a wildcard match to pattern string
func validateNumberWithStr(log logr.Logger, value interface{}, pattern string, operator operator.Operator) bool {
	typedValue, err := convertNumberToString(value)
	if err != nil {
		log.Error(err, "failed to convert to string")
		return false
	}

	patternQuan, err := apiresource.ParseQuantity(pattern)
	// 1. nil error - quantity comparison
	if err == nil {
		valueQuan, err := apiresource.ParseQuantity(typedValue)
		if err != nil {
			log.Error(err, "invalid quantity in resource", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
			return false
		}

		return compareQuantity(valueQuan, patternQuan, operator)
	}

	// 2. wildcard match
	if !wildcard.Match(pattern, typedValue) {
		log.V(4).Info("value failed wildcard check", "type", fmt.Sprintf("%T", typedValue), "value", typedValue, "check", pattern)
		return false
	}
	return true
}

func compareQuantity(value, pattern apiresource.Quantity, op operator.Operator) bool {
	result := value.Cmp(pattern)
	switch op {
	case operator.Equal:
		return result == int(equal)
	case operator.NotEqual:
		return result != int(equal)
	case operator.More:
		return result == int(greaterThan)
	case operator.Less:
		return result == int(lessThan)
	case operator.MoreEqual:
		return (result == int(equal)) || (result == int(greaterThan))
	case operator.LessEqual:
		return (result == int(equal)) || (result == int(lessThan))
	}

	return false
}

// detects numerical and string parts in pattern and returns them
func getNumberAndStringPartsFromPattern(pattern string) (number, str string) {
	regexpStr := `^(\d*(\.\d+)?)(.*)`
	re := regexp.MustCompile(regexpStr)
	matches := re.FindAllStringSubmatch(pattern, -1)
	match := matches[0]
	return match[1], match[3]
}
