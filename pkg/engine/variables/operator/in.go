package operator

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

// NewInHandler returns handler to manage In operations
//
// Deprecated: Use `NewAllInHandler` or `NewAnyInHandler` instead
func NewInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return InHandler{
		ctx: ctx,
		log: log,
	}
}

// InHandler provides implementation to handle In Operator
type InHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

// Evaluate evaluates expression with In Operator
func (in InHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return in.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64, bool:
		return in.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		return in.validateValueWithElementSetPattern(toElements(typedKey), value)
	default:
		in.log.V(2).Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (in InHandler) validateValueWithStringPattern(key string, value interface{}) (keyExists bool) {
	invalidType, keyExists := keyExistsInArray(key, value, in.log)
	if invalidType {
		in.log.V(2).Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

// keyExistsInArray checks if the  key exists in the array value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
func keyExistsInArray(key string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
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
		// A bare comparable scalar (int, float, bool) is treated as a
		// one-element set through the same primitive that coerces array
		// elements. An opaque value (null, map, nested object) has no
		// literal form to compare against, so its membership is undefined:
		// report an invalid type and let In and *NotIn both fail safe to
		// false, so a deny rule does not silently fire on absent or
		// non-scalar data (#1039).
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

func (in InHandler) validateValueWithElementSetPattern(key []element, value interface{}) (keyExists bool) {
	invalidType, isIn := setExistsInArray(key, value, in.log, false)
	if invalidType {
		in.log.V(2).Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isIn
}

// setExistsInArray checks if the key is a subset of value
// The value can be a string, an array of strings, or a JSON format
// array of strings (e.g. ["val1", "val2", "val3"].
// notIn argument if set to true will check for NotIn
func setExistsInArray(key []element, value interface{}, log logr.Logger, notIn bool) (invalidType bool, keyExists bool) {
	switch valuesAvailable := value.(type) {
	case []interface{}:
		valueElements := toElements(valuesAvailable)
		if notIn {
			return false, isNotIn(key, valueElements)
		}
		return false, isIn(key, valueElements)

	case string:
		if len(key) == 1 && equalElements(key[0], toElement(valuesAvailable)) {
			if notIn {
				return false, false
			}
			return false, true
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
		if notIn {
			return false, isNotIn(key, valueElements)
		}

		return false, isIn(key, valueElements)

	default:
		// Same rationale as keyExistsInArray's default case. An opaque
		// value (null, map, nested object) is not comparable, so report an
		// invalid type and let In/*NotIn fail safe to false (#1039).
		valueElement := toElement(value)
		if !valueElement.comparable {
			return true, false
		}
		valueElements := []element{valueElement}
		if notIn {
			return false, isNotIn(key, valueElements)
		}
		return false, isIn(key, valueElements)
	}
}

// isIn checks if all values in S1 are in S2 under exact equality
func isIn(key []element, value []element) bool {
	for _, k := range key {
		found := false
		for _, v := range value {
			if equalElements(k, v) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// isNotIn checks if any of the values in S1 is not in S2 under exact equality
func isNotIn(key []element, value []element) bool {
	for _, k := range key {
		found := false
		for _, v := range value {
			if equalElements(k, v) {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}

	return false
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
