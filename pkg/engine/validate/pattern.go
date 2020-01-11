package validate

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/minio/minio/pkg/wildcard"
	"github.com/nirmata/kyverno/pkg/engine/operator"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
)

type quantity int

const (
	equal       quantity = 0
	lessThan    quantity = -1
	greaterThan quantity = 1
)

// ValidateValueWithPattern validates value with operators and wildcards
func ValidateValueWithPattern(value, pattern interface{}) bool {
	switch typedPattern := pattern.(type) {
	case bool:
		typedValue, ok := value.(bool)
		if !ok {
			glog.V(4).Infof("Expected bool, found %T", value)
			return false
		}
		return typedPattern == typedValue
	case int:
		return validateValueWithIntPattern(value, int64(typedPattern))
	case int64:
		return validateValueWithIntPattern(value, typedPattern)
	case float64:
		return validateValueWithFloatPattern(value, typedPattern)
	case string:
		return validateValueWithStringPatterns(value, typedPattern)
	case nil:
		return validateValueWithNilPattern(value)
	case map[string]interface{}:
		// TODO: check if this is ever called?
		return validateValueWithMapPattern(value, typedPattern)
	case []interface{}:
		// TODO: check if this is ever called?
		glog.Warning("Arrays as patterns are not supported")
		return false
	default:
		glog.Warningf("Unknown type as pattern: %v", typedPattern)
		return false
	}
}

func validateValueWithMapPattern(value interface{}, typedPattern map[string]interface{}) bool {
	// verify the type of the resource value is map[string]interface,
	// we only check for existance of object, not the equality of content and value
	//TODO: check if adding
	_, ok := value.(map[string]interface{})
	if !ok {
		glog.Warningf("Expected map[string]interface{}, found %T\n", value)
		return false
	}
	return true
}

// Handler for int values during validation process
func validateValueWithIntPattern(value interface{}, pattern int64) bool {
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

		glog.Warningf("Expected int, found float: %f\n", typedValue)
		return false
	case string:
		// extract int64 from string
		int64Num, err := strconv.ParseInt(typedValue, 10, 64)
		if err != nil {
			glog.Warningf("Failed to parse int64 from string: %v", err)
			return false
		}
		return int64Num == pattern
	default:
		glog.Warningf("Expected int, found: %T\n", value)
		return false
	}
}

// Handler for float values during validation process
func validateValueWithFloatPattern(value interface{}, pattern float64) bool {
	switch typedValue := value.(type) {
	case int:
		// check that float has no fraction
		if pattern == math.Trunc(pattern) {
			return int(pattern) == value
		}
		glog.Warningf("Expected float, found int: %d\n", typedValue)
		return false
	case int64:
		// check that float has no fraction
		if pattern == math.Trunc(pattern) {
			return int64(pattern) == value
		}
		glog.Warningf("Expected float, found int: %d\n", typedValue)
		return false
	case float64:
		return typedValue == pattern
	case string:
		// extract float64 from string
		float64Num, err := strconv.ParseFloat(typedValue, 64)
		if err != nil {
			glog.Warningf("Failed to parse float64 from string: %v", err)
			return false
		}
		return float64Num == pattern
	default:
		glog.Warningf("Expected float, found: %T\n", value)
		return false
	}
}

// Handler for nil values during validation process
func validateValueWithNilPattern(value interface{}) bool {
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
		return typed == false
	case nil:
		return true
	case map[string]interface{}, []interface{}:
		glog.Warningf("Maps and arrays could not be checked with nil pattern")
		return false
	default:
		glog.Warningf("Unknown type as value when checking for nil pattern: %T\n", value)
		return false
	}
}

// Handler for pattern values during validation process
func validateValueWithStringPatterns(value interface{}, pattern string) bool {
	statements := strings.Split(pattern, "|")
	for _, statement := range statements {
		statement = strings.Trim(statement, " ")
		if validateValueWithStringPattern(value, statement) {
			return true
		}
	}

	return false
}

// Handler for single pattern value during validation process
// Detects if pattern has a number
func validateValueWithStringPattern(value interface{}, pattern string) bool {
	operator := operator.GetOperatorFromStringPattern(pattern)
	pattern = pattern[len(operator):]
	number, str := getNumberAndStringPartsFromPattern(pattern)

	if "" == number {
		return validateString(value, str, operator)
	}

	return validateNumberWithStr(value, pattern, operator)
}

// Handler for string values
func validateString(value interface{}, pattern string, operatorVariable operator.Operator) bool {
	if operator.NotEqual == operatorVariable || operator.Equal == operatorVariable {
		strValue, ok := value.(string)
		if !ok {
			glog.Warningf("Expected string, found %T\n", value)
			return false
		}

		wildcardResult := wildcard.Match(pattern, strValue)

		if operator.NotEqual == operatorVariable {
			return !wildcardResult
		}

		return wildcardResult
	}

	glog.Warningf("Operators >, >=, <, <= are not applicable to strings")
	return false
}

// validateNumberWithStr compares quantity if pattern type is quantity
//  or a wildcard match to pattern string
func validateNumberWithStr(value interface{}, pattern string, operator operator.Operator) bool {
	typedValue, err := convertToString(value)
	if err != nil {
		glog.Warning(err)
		return false
	}

	patternQuan, err := apiresource.ParseQuantity(pattern)
	// 1. nil error - quantity comparison
	if err == nil {
		valueQuan, err := apiresource.ParseQuantity(typedValue)
		if err != nil {
			glog.Warningf("Invalid quantity in resource %s, err: %v\n", typedValue, err)
			return false
		}

		return compareQuantity(valueQuan, patternQuan, operator)
	}

	// 2. wildcard match
	if !wildcard.Match(pattern, typedValue) {
		glog.Warningf("Value '%s' has not passed wildcard check: %s", typedValue, pattern)
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
