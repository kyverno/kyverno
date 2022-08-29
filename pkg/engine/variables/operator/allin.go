package operator

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
)

// NewAllInHandler returns handler to manage AllIn operations
func NewAllInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return AllInHandler{
		ctx: ctx,
		log: log,
	}
}

// AllInHandler provides implementation to handle AllIn Operator
type AllInHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

// Evaluate evaluates expression with AllIn Operator
func (allin AllInHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return allin.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64:
		return allin.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		var stringSlice []string
		for _, v := range typedKey {
			stringSlice = append(stringSlice, fmt.Sprint(v))
		}
		return allin.validateValueWithStringSetPattern(stringSlice, value)
	default:
		allin.log.V(2).Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (allin AllInHandler) validateValueWithStringPattern(key string, value interface{}) (keyExists bool) {
	invalidType, keyExists := allKeyExistsInArray(key, value, allin.log)
	if invalidType {
		allin.log.V(2).Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

func allKeyExistsInArray(key string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
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
		return true, false
	}

	return false, false
}

func (allin AllInHandler) validateValueWithStringSetPattern(key []string, value interface{}) (keyExists bool) {
	invalidType, isAllIn := allSetExistsInArray(key, value, allin.log, false)
	if invalidType {
		allin.log.V(2).Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isAllIn
}

// allsetExistsInArray checks if all key is a subset of value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
// allnotIn argument if set to true will check for allNotIn
func allSetExistsInArray(key []string, value interface{}, log logr.Logger, allNotIn bool) (invalidType bool, keyExists bool) {
	switch valuesAvailable := value.(type) {
	case []interface{}:
		var valueSlice []string
		for _, val := range valuesAvailable {
			valueSlice = append(valueSlice, fmt.Sprint(val))
		}
		if allNotIn {
			return false, isAllNotIn(key, valueSlice)
		}
		return false, isAllIn(key, valueSlice)

	case string:

		if len(key) == 1 && key[0] == valuesAvailable {
			if allNotIn {
				return false, false
			}
			return false, true
		}

		operatorVariable := operator.GetOperatorFromStringPattern(fmt.Sprintf("%v", value))
		if operatorVariable == operator.InRange {
			if allNotIn {
				isAllNotInBool := true
				for _, k := range key {
					if handleRange(k, valuesAvailable, log) {
						isAllNotInBool = false
					}
				}
				return false, isAllNotInBool
			} else {
				isAllInCount := 0
				for _, k := range key {
					if handleRange(k, value, log) {
						isAllInCount++
					}
				}
				if isAllInCount == len(key) {
					return false, true
				}
				return false, false
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
		if allNotIn {
			return false, isAllNotIn(key, arr)
		}

		return false, isAllIn(key, arr)

	default:
		return true, false
	}
}

// isAllIn checks if all values in S1 are in S2
func isAllIn(key []string, value []string) bool {
	found := 0
	for _, valKey := range key {
		for _, valValue := range value {
			if wildcard.Match(valKey, valValue) || wildcard.Match(valValue, valKey) {
				found++
				break
			}
		}
	}
	return found == len(key)
}

// isAllNotIn checks if all the values in S1 are not in S2
func isAllNotIn(key []string, value []string) bool {
	found := 0
	for _, valKey := range key {
		for _, valValue := range value {
			if wildcard.Match(valKey, valValue) || wildcard.Match(valValue, valKey) {
				found++
				break
			}
		}
	}
	return found != len(key)
}

func (allin AllInHandler) validateValueWithBoolPattern(_ bool, _ interface{}) bool {
	return false
}

func (allin AllInHandler) validateValueWithIntPattern(_ int64, _ interface{}) bool {
	return false
}

func (allin AllInHandler) validateValueWithFloatPattern(_ float64, _ interface{}) bool {
	return false
}

func (allin AllInHandler) validateValueWithMapPattern(_ map[string]interface{}, _ interface{}) bool {
	return false
}

func (allin AllInHandler) validateValueWithSlicePattern(_ []interface{}, _ interface{}) bool {
	return false
}
