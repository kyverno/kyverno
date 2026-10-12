package operator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	"github.com/kyverno/kyverno/pkg/engine/pattern"
)

// NewAnyInHandler returns handler to manage AnyIn operations
func NewAnyInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return AnyInHandler{
		ctx: ctx,
		log: log,
	}
}

// AnyInHandler provides implementation to handle AnyIn Operator
type AnyInHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

// Evaluate evaluates expression with AnyIn Operator
func (anyin AnyInHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return anyin.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64, bool:
		return anyin.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		return anyin.validateValueWithElementSetPattern(toElements(typedKey), value)
	default:
		anyin.log.V(2).Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (anyin AnyInHandler) validateValueWithStringPattern(key string, value interface{}) (keyExists bool) {
	invalidType, keyExists := anyKeyExistsInArray(key, value, anyin.log)
	if invalidType {
		anyin.log.V(2).Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

// anyKeyExistsInArray checks if the  key exists in the array value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
func anyKeyExistsInArray(key string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
	keyElement := toElement(key)

	switch valuesAvailable := value.(type) {
	case []interface{}:
		for _, val := range valuesAvailable {
			if wildcardElements(toElement(val), keyElement) {
				return false, true
			}
		}

	case string:
		if wildcardElementDirectional(toElement(valuesAvailable), keyElement) {
			return false, true
		}

		operatorVariable := operator.GetOperatorFromStringPattern(fmt.Sprintf("%v", value))
		if operatorVariable == operator.InRange {
			return false, handleRange(key, value, log)
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

		for _, val := range arr {
			if key == val {
				return false, true
			}
		}

	default:
		// A bare comparable scalar is a one-element set; an opaque value
		// (null, map, nested object) is not comparable, so fail safe to an
		// invalid type (In/*NotIn both false) rather than firing a deny on
		// absent or non-scalar data (#1039).
		valueElement := toElement(valuesAvailable)
		if !valueElement.comparable {
			return true, false
		}
		// Directional, matching the scalar-value semantics above: value
		// is the pattern, key is the concrete string.
		if wildcardElementDirectional(valueElement, keyElement) {
			return false, true
		}
	}

	return false, false
}

func handleRange(key string, value interface{}, log logr.Logger) bool {
	if !pattern.Validate(log, key, value) {
		return false
	} else {
		return true
	}
}

func (anyin AnyInHandler) validateValueWithElementSetPattern(key []element, value interface{}) (keyExists bool) {
	invalidType, isAnyIn := anySetExistsInArray(key, value, anyin.log, false)
	if invalidType {
		anyin.log.V(2).Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isAnyIn
}

// anySetExistsInArray checks if any key is a subset of value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
// notIn argument if set to true will check for NotIn
func anySetExistsInArray(key []element, value interface{}, log logr.Logger, anyNotIn bool) (invalidType bool, keyExists bool) {
	switch valuesAvailable := value.(type) {
	case []interface{}:
		valueElements := toElements(valuesAvailable)
		if anyNotIn {
			return false, isAnyNotIn(key, valueElements)
		}
		return false, isAnyIn(key, valueElements)

	case int, int32, int64, float32, float64, bool:
		valueElements := []element{toElement(value)}
		if anyNotIn {
			return false, isAnyNotIn(key, valueElements)
		}
		return false, isAnyIn(key, valueElements)

	case string:
		if len(key) == 1 && equalElements(key[0], toElement(valuesAvailable)) {
			if anyNotIn {
				return false, false
			}
			return false, true
		}

		operatorVariable := operator.GetOperatorFromStringPattern(fmt.Sprintf("%v", value))
		if operatorVariable == operator.InRange {
			if anyNotIn {
				isAnyNotInBool := false
				stringForAnyNotIn := strings.Replace(valuesAvailable, "-", "!-", 1)
				for _, k := range key {
					if handleRange(k.str, stringForAnyNotIn, log) {
						isAnyNotInBool = true
						break
					}
				}
				return false, isAnyNotInBool
			} else {
				isAnyInBool := false
				for _, k := range key {
					if handleRange(k.str, value, log) {
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
		valueElements := elementsFromStrings(arr)
		if anyNotIn {
			return false, isAnyNotIn(key, valueElements)
		}

		return false, isAnyIn(key, valueElements)

	default:
		// Same rationale as anyKeyExistsInArray's default case. An opaque
		// value (null, map, nested object) is not comparable, so report an
		// invalid type and fail safe to false (#1039).
		valueElement := toElement(value)
		if !valueElement.comparable {
			return true, false
		}
		valueElements := []element{valueElement}
		if anyNotIn {
			return false, isAnyNotIn(key, valueElements)
		}
		return false, isAnyIn(key, valueElements)
	}
}

// isAnyIn checks if any values in S1 are in S2 under wildcard matching
func isAnyIn(key []element, value []element) bool {
	for _, valKey := range key {
		for _, valValue := range value {
			if wildcardElements(valKey, valValue) {
				return true
			}
		}
	}
	return false
}

// isAnyNotIn checks if any of the values in S1 are not in S2 under wildcard matching
func isAnyNotIn(key []element, value []element) bool {
	for _, valKey := range key {
		matchFound := false

		for _, valValue := range value {
			if wildcardElements(valKey, valValue) {
				matchFound = true
				break
			}
		}

		if !matchFound {
			return true
		}
	}
	return false
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
