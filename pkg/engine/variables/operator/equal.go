package operator

import (
	"fmt"
	"math"
	"reflect"
	"strconv"

	"github.com/minio/pkg/wildcard"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewEqualHandler returns handler to manage Equal operations
func NewEqualHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return EqualHandler{
		ctx: ctx,
		log: log,
	}
}

//EqualHandler provides implementation to handle NotEqual Operator
type EqualHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

//Evaluate evaluates expression with Equal Operator
func (eh EqualHandler) Evaluate(key, value interface{}) bool {
	// key and value need to be of same type
	switch typedKey := key.(type) {
	case bool:
		return eh.validateValueWithBoolPattern(typedKey, value)
	case int:
		return eh.validateValueWithIntPattern(int64(typedKey), value)
	case int64:
		return eh.validateValueWithIntPattern(typedKey, value)
	case float64:
		return eh.validateValueWithFloatPattern(typedKey, value)
	case string:
		return eh.validateValueWithStringPattern(typedKey, value)
	case map[string]interface{}:
		return eh.validateValueWithMapPattern(typedKey, value)
	case []interface{}:
		return eh.validateValueWithSlicePattern(typedKey, value)
	default:
		eh.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (eh EqualHandler) validateValueWithSlicePattern(key []interface{}, value interface{}) bool {
	if val, ok := value.([]interface{}); ok {
		return reflect.DeepEqual(key, val)
	}
	eh.log.Info("Expected type []interface{}", "value", value, "type", fmt.Sprintf("%T", value))
	return false
}

func (eh EqualHandler) validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool {
	if val, ok := value.(map[string]interface{}); ok {
		return reflect.DeepEqual(key, val)
	}
	eh.log.Info("Expected type map[string]interface{}", "value", value, "type", fmt.Sprintf("%T", value))
	return false
}

func (eh EqualHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	// We need to check duration first as it's the only type that can be compared to a different type.
	durationKey, durationValue, err := parseDuration(key, value)
	if err == nil {
		return durationKey.Seconds() == durationValue.Seconds()
	}

	// Attempt to extract resource quantity from string.
	resourceKey, err := resource.ParseQuantity(key)
	if err == nil {
		switch typedValue := value.(type) {
		case string:
			resourceValue, err := resource.ParseQuantity(typedValue)
			if err != nil {
				eh.log.Error(fmt.Errorf("parse error: "), "Failed to parse value type doesn't match key type")
				return false
			}
			return resourceKey.Equal(resourceValue)
		}
	}

	if val, ok := value.(string); ok {
		return wildcard.Match(val, key)
	}

	eh.log.Info("Expected type string", "value", value, "type", fmt.Sprintf("%T", value))
	return false
}

func (eh EqualHandler) validateValueWithFloatPattern(key float64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		// check that float has not fraction
		if key == math.Trunc(key) {
			return int(key) == typedValue
		}
		eh.log.Info("Expected type float, found int", "typedValue", typedValue)
	case int64:
		// check that float has not fraction
		if key == math.Trunc(key) {
			return int64(key) == typedValue
		}
		eh.log.Info("Expected type float, found int", "typedValue", typedValue)
	case float64:
		return typedValue == key
	case string:
		// extract float from string
		float64Num, err := strconv.ParseFloat(typedValue, 64)
		if err != nil {
			eh.log.Error(err, "Failed to parse float64 from string")
			return false
		}
		return float64Num == key
	default:
		eh.log.Info("Expected type float", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
	return false
}

func (eh EqualHandler) validateValueWithBoolPattern(key bool, value interface{}) bool {
	typedValue, ok := value.(bool)
	if !ok {
		eh.log.Info("Expected type bool", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
	return key == typedValue
}

func (eh EqualHandler) validateValueWithIntPattern(key int64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return int64(typedValue) == key
	case int64:
		return typedValue == key
	case float64:
		// check that float has no fraction
		if typedValue == math.Trunc(typedValue) {
			return int64(typedValue) == key
		}
		eh.log.Info("Expected type int, found float", "value", typedValue, "type", fmt.Sprintf("%T", typedValue))
		return false
	case string:
		// extract in64 from string
		int64Num, err := strconv.ParseInt(typedValue, 10, 64)
		if err != nil {
			eh.log.Error(err, "Failed to parse int64 from string")
			return false
		}
		return int64Num == key
	default:
		eh.log.Info("Expected type int", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}
