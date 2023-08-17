package pattern

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
)

type quantity int

const (
	equal       quantity = 0
	lessThan    quantity = -1
	greaterThan quantity = 1
)

// Validate validates a value against a pattern
func Validate(log logr.Logger, value, pattern interface{}) bool {
	switch typedPattern := pattern.(type) {
	case bool:
		return validateBoolPattern(log, value, typedPattern)
	case int:
		return validateIntPattern(log, value, int64(typedPattern))
	case int64:
		return validateIntPattern(log, value, typedPattern)
	case float64:
		return validateFloatPattern(log, value, typedPattern)
	case nil:
		return validateNilPattern(log, value)
	case map[string]interface{}:
		return validateMapPattern(log, value, typedPattern)
	case string:
		return validateStringPatterns(log, value, typedPattern)
	case []interface{}:
		log.V(2).Info("arrays are not supported as patterns")
		return false
	default:
		log.V(2).Info("Unknown type", "type", fmt.Sprintf("%T", typedPattern), "value", typedPattern)
		return false
	}
}

func validateBoolPattern(log logr.Logger, value interface{}, pattern bool) bool {
	switch typedValue := value.(type) {
	case bool:
		return pattern == typedValue
	default:
		log.V(4).Info("Expected type bool", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

func validateIntPattern(log logr.Logger, value interface{}, pattern int64) bool {
	switch typedValue := value.(type) {
	case int:
		return int64(typedValue) == pattern
	case int64:
		return typedValue == pattern
	case float64:
		// check that float has no fraction
		if typedValue != math.Trunc(typedValue) {
			log.V(2).Info("Expected type int", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
			return false
		}
		return int64(typedValue) == pattern
	case string:
		value, err := strconv.ParseInt(typedValue, 10, 64)
		if err != nil {
			log.Error(err, "Failed to parse int64 from string")
			return false
		}
		return value == pattern
	default:
		log.V(2).Info("Expected type int", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

func validateFloatPattern(log logr.Logger, value interface{}, pattern float64) bool {
	switch typedValue := value.(type) {
	case int:
		// check that float has no fraction
		if pattern != math.Trunc(pattern) {
			log.V(2).Info("Expected type float", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
			return false
		}
		return int(pattern) == value
	case int64:
		// check that float has no fraction
		if pattern != math.Trunc(pattern) {
			log.V(2).Info("Expected type float", "type", fmt.Sprintf("%T", typedValue), "value", typedValue)
			return false
		}
		return int64(pattern) == value
	case float64:
		return typedValue == pattern
	case string:
		value, err := strconv.ParseFloat(typedValue, 64)
		if err != nil {
			log.Error(err, "Failed to parse float64 from string")
			return false
		}
		return value == pattern
	default:
		log.V(2).Info("Expected type float", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

func validateNilPattern(log logr.Logger, value interface{}) bool {
	switch typedValue := value.(type) {
	case float64:
		return typedValue == 0.0
	case int:
		return typedValue == 0
	case int64:
		return typedValue == 0
	case string:
		return typedValue == ""
	case bool:
		return !typedValue
	case nil:
		return true
	case map[string]interface{}, []interface{}:
		log.V(2).Info("Maps and arrays could not be checked with nil pattern")
		return false
	default:
		log.V(2).Info("Unknown type as value when checking for nil pattern", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
}

func validateMapPattern(log logr.Logger, value interface{}, _ map[string]interface{}) bool {
	// verify the type of the resource value is map[string]interface,
	// we only check for existence of object, not the equality of content and value
	_, ok := value.(map[string]interface{})
	if !ok {
		log.V(2).Info("Expected type map[string]interface{}", "type", fmt.Sprintf("%T", value), "value", value)
		return false
	}
	return true
}

func validateStringPatterns(log logr.Logger, value interface{}, pattern string) bool {
	if value == pattern {
		return true
	}
	for _, condition := range strings.Split(pattern, "|") {
		condition = strings.Trim(condition, " ")
		if checkForAndConditionsAndValidate(log, value, condition) {
			return true
		}
	}
	return false
}

func checkForAndConditionsAndValidate(log logr.Logger, value interface{}, pattern string) bool {
	for _, condition := range strings.Split(pattern, "&") {
		condition = strings.Trim(condition, " ")
		if !validateStringPattern(log, value, condition) {
			return false
		}
	}
	return true
}

func validateStringPattern(log logr.Logger, value interface{}, pattern string) bool {
	op := operator.GetOperatorFromStringPattern(pattern)
	if op == operator.InRange {
		// Upon encountering InRange operator split the string by `-` and basically
		// verify the result of (x >= leftEndpoint & x <= rightEndpoint)
		if left, right, ok := split(pattern, operator.InRangeRegex); ok {
			return validateStringPattern(log, value, fmt.Sprintf(">= %s", left)) &&
				validateStringPattern(log, value, fmt.Sprintf("<= %s", right))
		}
		return false
	} else if op == operator.NotInRange {
		// Upon encountering NotInRange operator split the string by `!-` and basically
		// verify the result of (x < leftEndpoint | x > rightEndpoint)
		if left, right, ok := split(pattern, operator.NotInRangeRegex); ok {
			return validateStringPattern(log, value, fmt.Sprintf("< %s", left)) ||
				validateStringPattern(log, value, fmt.Sprintf("> %s", right))
		}
		return false
	} else {
		pattern := strings.TrimSpace(pattern[len(op):])
		return validateString(log, value, pattern, op)
	}
}

func split(pattern string, r *regexp.Regexp) (string, string, bool) {
	match := r.FindStringSubmatch(pattern)
	if len(match) == 0 {
		return "", "", false
	}
	return match[1], match[2], true
}

func validateString(log logr.Logger, value interface{}, pattern string, op operator.Operator) bool {
	if res, proc := compareDuration(log, value, pattern, op); proc {
		return res
	}
	if res, proc := compareQuantity(log, value, pattern, op); proc {
		return res
	}
	return compareString(log, value, pattern, op)
}

func compareDuration(_ logr.Logger, value interface{}, pattern string, op operator.Operator) (res bool, processed bool) {
	if pattern, err := time.ParseDuration(pattern); err != nil {
		return false, false
	} else if value, err := convertNumberToString(value); err != nil {
		return false, false
	} else if value, err := time.ParseDuration(value); err != nil {
		return false, false
	} else {
		switch op {
		case operator.Equal:
			return value == pattern, true
		case operator.NotEqual:
			return value != pattern, true
		case operator.More:
			return value > pattern, true
		case operator.Less:
			return value < pattern, true
		case operator.MoreEqual:
			return value >= pattern, true
		case operator.LessEqual:
			return value <= pattern, true
		}
		return false, false
	}
}

func compareQuantity(_ logr.Logger, value interface{}, pattern string, op operator.Operator) (res bool, processed bool) {
	if pattern, err := apiresource.ParseQuantity(pattern); err != nil {
		return false, false
	} else if value, err := convertNumberToString(value); err != nil {
		return false, false
	} else if value, err := apiresource.ParseQuantity(value); err != nil {
		return false, false
	} else {
		result := value.Cmp(pattern)
		switch op {
		case operator.Equal:
			return result == int(equal), true
		case operator.NotEqual:
			return result != int(equal), true
		case operator.More:
			return result == int(greaterThan), true
		case operator.Less:
			return result == int(lessThan), true
		case operator.MoreEqual:
			return (result == int(equal)) || (result == int(greaterThan)), true
		case operator.LessEqual:
			return (result == int(equal)) || (result == int(lessThan)), true
		}
		return false, false
	}
}

func compareString(log logr.Logger, value interface{}, pattern string, operatorVariable operator.Operator) bool {
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
	log.V(2).Info("Operators >, >=, <, <= are not applicable to strings")
	return false
}

func convertNumberToString(value interface{}) (string, error) {
	if value == nil {
		return "0", nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case float64:
		return fmt.Sprintf("%f", typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.Itoa(typed), nil
	default:
		return "", fmt.Errorf("could not convert %v to string", typed)
	}
}
