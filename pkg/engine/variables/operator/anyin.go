package operator

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
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
	invalidType, keyExists := keyExistsInArray(key, value, anyin.log)
	if invalidType {
		anyin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
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

		if len(key) == 1 && key[0] == valuesAvailable {
			return false, true
		}

		var arr []string
		if err := json.Unmarshal([]byte(valuesAvailable), &arr); err != nil {
			log.Error(err, "failed to unmarshal value to JSON string array", "key", key, "value", value)
			return true, false
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
			if wildcard.Match(valKey, valValue) {
				return true
			}
		}
	}
	return false
}

// isAllNotIn checks if all the values in S1 are not in S2
func isAnyNotIn(key []string, value []string) bool {
	found := 0
	for _, valKey := range key {
		for _, valValue := range value {
			if wildcard.Match(valKey, valValue) {
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
