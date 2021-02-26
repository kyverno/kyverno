package operator

import (
	"encoding/json"
	"fmt"

	"github.com/minio/minio/pkg/wildcard"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewInHandler returns handler to manage In operations
func NewInHandler(log logr.Logger, ctx context.EvalInterface, subHandler VariableSubstitutionHandler) OperatorHandler {
	return InHandler{
		ctx:        ctx,
		subHandler: subHandler,
		log:        log,
	}
}

//InHandler provides implementation to handle In Operator
type InHandler struct {
	ctx        context.EvalInterface
	subHandler VariableSubstitutionHandler
	log        logr.Logger
}

//Evaluate evaluates expression with In Operator
func (in InHandler) Evaluate(key, value interface{}) bool {
	var err error
	// substitute the variables
	if key, err = in.subHandler(in.log, in.ctx, key); err != nil {
		in.log.Error(err, "Failed to resolve variable", "variable", key)
		return false
	}

	if value, err = in.subHandler(in.log, in.ctx, value); err != nil {
		in.log.Error(err, "Failed to resolve variable", "variable", value)
		return false
	}

	switch typedKey := key.(type) {
	case string:
		return in.validateValueWithStringPattern(typedKey, value)
	case []interface{}:
		var stringSlice []string
		for _, v := range typedKey {
			stringSlice = append(stringSlice, v.(string))
		}
		return in.validateValueWithStringSetPattern(stringSlice, value)
	default:
		in.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (in InHandler) validateValueWithStringPattern(key string, value interface{}) (keyExists bool) {
	invalidType, keyExists := keyExistsInArray(key, value, in.log)
	if invalidType {
		in.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

// keyExistsInArray checks if the  key exists in the array value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
func keyExistsInArray(key string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
	switch valuesAvailable := value.(type) {

	case []interface{}:
		for _, val := range valuesAvailable {
			v, ok := val.(string)
			if !ok {
				return true, false
			}

			if ok && wildcard.Match(key, v) {
				return false, true
			}
		}

	case string:

		if wildcard.Match(valuesAvailable, key) {
			return false, true
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

func (in InHandler) validateValueWithStringSetPattern(key []string, value interface{}) (keyExists bool) {
	invalidType, keyExists := setExistsInArray(key, value, in.log)
	if invalidType {
		in.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

// setExistsInArray checks if the key is a subset of value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
func setExistsInArray(key []string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
	switch valuesAvailable := value.(type) {

	case []interface{}:
		var valueSlice []string
		for _, val := range valuesAvailable {
			v, ok := val.(string)
			if !ok {
				return true, false
			}
			valueSlice = append(valueSlice, v)
		}

		return false, isSubset(key, valueSlice)

	case string:

		if len(key) == 1 && key[0] == valuesAvailable {
			return false, true
		}

		var arr []string
		if err := json.Unmarshal([]byte(valuesAvailable), &arr); err != nil {
			log.Error(err, "failed to unmarshal value to JSON string array", "key", key, "value", value)
			return true, false
		}

		return false, isSubset(key, arr)

	default:
		return true, false
	}
}

// isSubset checks if S1 is a subset of S2 i.e. ALL values of S1 are in S2
func isSubset(key []string, value []string) bool {
	set := make(map[string]int)

	for _, val := range value {
		set[val]++
	}

	for _, val := range key {
		count, found := set[val]
		if !found {
			return false
		} else if count < 1 {
			return false
		} else {
			set[val] = count - 1
		}
	}

	return true
}

func (in InHandler) validateValueWithBoolPattern(_ bool, _ interface{}) bool {
	return false
}

func (in InHandler) validateValueWithIntPattern(_ int64, _ interface{}) bool {
	return false
}

func (in InHandler) validateValueWithFloatPattern(_ float64, _ interface{}) bool {
	return false
}

func (in InHandler) validateValueWithMapPattern(_ map[string]interface{}, _ interface{}) bool {
	return false
}

func (in InHandler) validateValueWithSlicePattern(_ []interface{}, _ interface{}) bool {
	return false
}
