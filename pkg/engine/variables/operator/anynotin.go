package operator

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewAnyNotInHandler returns handler to manage AnyNotIn operations
func NewAnyNotInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return NotInHandler{
		ctx: ctx,
		log: log,
	}
}

//AnyNotInHandler provides implementation to handle AnyNotIn Operator
type AnyNotInHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

//Evaluate evaluates expression with AnyNotIn Operator
func (anynin AnyNotInHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return anynin.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64:
		return anynin.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		var stringSlice []string
		for _, v := range typedKey {
			stringSlice = append(stringSlice, fmt.Sprint(v))
		}
		return anynin.validateValueWithStringSetPattern(stringSlice, value)
	default:
		anynin.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (anynin AnyNotInHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	invalidType, keyExists := keyExistsInArray(key, value, anynin.log)
	if invalidType {
		anynin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return !keyExists
}

func (anynin AnyNotInHandler) validateValueWithStringSetPattern(key []string, value interface{}) bool {
	invalidType, isAnyNotIn := anySetExistsInArray(key, value, anynin.log, true)
	if invalidType {
		anynin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isAnyNotIn
}

func (anynin AnyNotInHandler) validateValueWithBoolPattern(_ bool, _ interface{}) bool {
	return false
}

func (anynin AnyNotInHandler) validateValueWithIntPattern(_ int64, _ interface{}) bool {
	return false
}

func (anynin AnyNotInHandler) validateValueWithFloatPattern(_ float64, _ interface{}) bool {
	return false
}

func (anynin AnyNotInHandler) validateValueWithMapPattern(_ map[string]interface{}, _ interface{}) bool {
	return false
}

func (anynin AnyNotInHandler) validateValueWithSlicePattern(_ []interface{}, _ interface{}) bool {
	return false
}
