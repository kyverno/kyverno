package operator

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewNotInHandler returns handler to manage NotIn operations
//
// Deprecated: Use `NewAllNotInHandler` or `NewAnyNotInHandler` instead
func NewNotInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return NotInHandler{
		ctx: ctx,
		log: log,
	}
}

// NotInHandler provides implementation to handle NotIn Operator
type NotInHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

// Evaluate evaluates expression with NotIn Operator
func (nin NotInHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return nin.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64:
		return nin.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		var stringSlice []string
		for _, v := range typedKey {
			stringSlice = append(stringSlice, v.(string))
		}
		return nin.validateValueWithStringSetPattern(stringSlice, value)
	default:
		nin.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (nin NotInHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	invalidType, keyExists := keyExistsInArray(key, value, nin.log)
	if invalidType {
		nin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return !keyExists
}

func (nin NotInHandler) validateValueWithStringSetPattern(key []string, value interface{}) bool {
	invalidType, isNotIn := setExistsInArray(key, value, nin.log, true)
	if invalidType {
		nin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isNotIn
}

func (nin NotInHandler) validateValueWithBoolPattern(_ bool, _ interface{}) bool {
	return false
}

func (nin NotInHandler) validateValueWithIntPattern(_ int64, _ interface{}) bool {
	return false
}

func (nin NotInHandler) validateValueWithFloatPattern(_ float64, _ interface{}) bool {
	return false
}

func (nin NotInHandler) validateValueWithMapPattern(_ map[string]interface{}, _ interface{}) bool {
	return false
}

func (nin NotInHandler) validateValueWithSlicePattern(_ []interface{}, _ interface{}) bool {
	return false
}
