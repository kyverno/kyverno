package operator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	apiresource "k8s.io/apimachinery/pkg/api/resource"

	"github.com/minio/pkg/wildcard"
)

//NewAnyInHandler returns handler to manage AnyIn operations
func NewAnyInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return AnyInHandler{
		ctx: ctx,
		log: log,
	}
}

//AnyInHandler provides implementation to handle AnyIn Operator
type AnyInHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

//Evaluate evaluates expression with AnyIn Operator
func (anyin AnyInHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return anyin.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64:
		return anyin.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		var stringSlice []string
		for _, v := range typedKey {
			stringSlice = append(stringSlice, fmt.Sprint(v))
		}
		return anyin.validateValueWithStringSetPattern(stringSlice, value)
	default:
		anyin.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (anyin AnyInHandler) validateValueWithStringPattern(key string, value interface{}) (keyExists bool) {
	invalidType, keyExists := anyKeyExistsInArray(key, value, anyin.log)
	if invalidType {
		anyin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

// anykeyExistsInArray checks if the  key exists in the array value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
func anyKeyExistsInArray(key string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
	switch valuesAvailable := value.(type) {

	case []interface{}:
		for _, val := range valuesAvailable {
			if wildcard.Match(key, fmt.Sprint(val)) {
				return false, true
			}
		}

	case string:
		if wildcard.Match(valuesAvailable, key) {
			return false, true
		}

		operatorVariable := operator.GetOperatorFromStringPattern(fmt.Sprintf("%v", value))
		if operatorVariable == operator.InRange {
			return false, handleRange(key, value, log)
		}

		var arr []string
		if err := json.Unmarshal([]byte(valuesAvailable), &arr); err != nil {
			log.Error(err, "failed to unmarshal value to JSON string array", "key", key, "value", value)
			return true, false
		}

		for _, val := range arr {
			if key == val {
				return false, true
			}
		}

	default:
		invalidType = true
		return
	}

	return false, false
}

func handleRange(key string, value interface{}, log logr.Logger) bool {
	if !ValidateValueWithPattern(log, key, value) {
		return false
	} else {
		return true
	}
}

func (anyin AnyInHandler) validateValueWithStringSetPattern(key []string, value interface{}) (keyExists bool) {
	invalidType, isAnyIn := anySetExistsInArray(key, value, anyin.log, false)
	if invalidType {
		anyin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isAnyIn
}

// anysetExistsInArray checks if any key is a subset of value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
// notIn argument if set to true will check for NotIn
func anySetExistsInArray(key []string, value interface{}, log logr.Logger, anyNotIn bool) (invalidType bool, keyExists bool) {
	fmt.Println("enter anyset \n", anyNotIn)
	switch valuesAvailable := value.(type) {

	case []interface{}:
		var valueSlice []string
		for _, val := range valuesAvailable {
			valueSlice = append(valueSlice, fmt.Sprint(val))
		}
		if anyNotIn {
			return false, isAnyNotIn(key, valueSlice)
		}
		return false, isAnyIn(key, valueSlice)

	case string:
		fmt.Println("enter string value")
		if len(key) == 1 && key[0] == valuesAvailable {
			return false, true
		}

		operatorVariable := operator.GetOperatorFromStringPattern(fmt.Sprintf("%v", value))
		if operatorVariable == operator.InRange {
			if anyNotIn {
				fmt.Println("enter anynotin")
				isAnyNotInBool := false
				stringForAnyNotIn := strings.Replace(valuesAvailable, "-", "!-", 1)
				fmt.Println(stringForAnyNotIn)
				for _, k := range key {
					if handleRange(k, stringForAnyNotIn, log) {
						isAnyNotInBool = true
						break
					}
				}
				return false, isAnyNotInBool
			} else {
				isAnyInBool := false
				for _, k := range key {
					if handleRange(k, value, log) {
						isAnyInBool = true
						break
					}
				}
				return false, isAnyInBool
			}
		}

		var arr []string
		if json.Valid([]byte(valuesAvailable)) {
			if err := json.Unmarshal([]byte(valuesAvailable), &arr); err != nil {
				log.Error(err, "failed to unmarshal value to JSON string array", "key", key, "value", value)
				return true, false
			}
		} else {
			arr = append(arr, valuesAvailable)
		}
		if anyNotIn {
			return false, isAnyNotIn(key, arr)
		}

		return false, isAnyIn(key, arr)

	default:
		return true, false
	}
}

// isAnyIn checks if any values in S1 are in S2
func isAnyIn(key []string, value []string) bool {
	for _, valKey := range key {
		for _, valValue := range value {
			if wildcard.Match(valKey, valValue) || wildcard.Match(valValue, valKey) {
				return true
			}
		}
	}
	return false
}

// isAnyNotIn checks if any of the values in S1 are not in S2
func isAnyNotIn(key []string, value []string) bool {
	found := 0
	for _, valKey := range key {
		for _, valValue := range value {
			if wildcard.Match(valKey, valValue) || wildcard.Match(valValue, valKey) {
				found++
				break
			}
		}
	}
	return found < len(key)
}

func (anyin AnyInHandler) validateValueWithBoolPattern(_ bool, _ interface{}) bool {
	return false
}

func (anyin AnyInHandler) validateValueWithIntPattern(_ int64, _ interface{}) bool {
	return false
}

func (anyin AnyInHandler) validateValueWithFloatPattern(_ float64, _ interface{}) bool {
	return false
}

func (anyin AnyInHandler) validateValueWithMapPattern(_ map[string]interface{}, _ interface{}) bool {
	return false
}

func (anyin AnyInHandler) validateValueWithSlicePattern(_ []interface{}, _ interface{}) bool {
	return false
}

// ValidateValueWithPattern validates value with operators and wildcards
func ValidateValueWithPattern(log logr.Logger, value, pattern interface{}) bool {
	switch typedPattern := pattern.(type) {
	case string:
		return validateValueWithStringPatterns(log, value, typedPattern)
	case []interface{}:
		log.Info("arrays are not supported as patterns")
		return false
	default:
		log.Info("Unknown type", "type", fmt.Sprintf("%T", typedPattern), "value", typedPattern)
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

// detects numerical and string parts in pattern and returns them
func getNumberAndStringPartsFromPattern(pattern string) (number, str string) {
	regexpStr := `^(\d*(\.\d+)?)(.*)`
	re := regexp.MustCompile(regexpStr)
	matches := re.FindAllStringSubmatch(pattern, -1)
	match := matches[0]
	return match[1], match[3]
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

// convertNumberToString converts value to string
func convertNumberToString(value interface{}) (string, error) {
	if value == nil {
		return "0", nil
	}

	switch typed := value.(type) {
	case string:
		return string(typed), nil
	case float64:
		return fmt.Sprintf("%f", typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.Itoa(typed), nil
	case nil:
		return "", fmt.Errorf("got empty string, expect %v", value)
	default:
		return "", fmt.Errorf("could not convert %v to string", typed)
	}
}

type quantity int

const (
	equal       quantity = 0
	lessThan    quantity = -1
	greaterThan quantity = 1
)

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
