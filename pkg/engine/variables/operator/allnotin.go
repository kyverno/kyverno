package operator

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewAllNotInHandler returns handler to manage AllNotIn operations
func NewAllNotInHandler(log logr.Logger, ctx context.EvalInterface) OperatorHandler {
	return AllNotInHandler{
		ctx: ctx,
		log: log,
	}
}

//AllNotInHandler provides implementation to handle AllNotIn Operator
type AllNotInHandler struct {
	ctx context.EvalInterface
	log logr.Logger
}

//Evaluate evaluates expression with AllNotIn Operator
func (allnin AllNotInHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case string:
		return allnin.validateValueWithStringPattern(typedKey, value)
	case int, int32, int64, float32, float64:
		return allnin.validateValueWithStringPattern(fmt.Sprint(typedKey), value)
	case []interface{}:
		var stringSlice []string
		for _, v := range typedKey {
			stringSlice = append(stringSlice, fmt.Sprint(v))
		}
		return allnin.validateValueWithStringSetPattern(stringSlice, value)
	default:
		allnin.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (allnin AllNotInHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	invalidType, keyExists := keyExistsInArray(key, value, allnin.log)
	if invalidType {
		allnin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return !keyExists
}

func (allnin AllNotInHandler) validateValueWithStringSetPattern(key []string, value interface{}) bool {
	invalidType, isNotIn := allSetExistsInArray(key, value, allnin.log, true)
	if invalidType {
		allnin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return isNotIn
}

func (allnin AllNotInHandler) validateValueWithBoolPattern(_ bool, _ interface{}) bool {
	return false
}

func (allnin AllNotInHandler) validateValueWithIntPattern(_ int64, _ interface{}) bool {
	return false
}

func (allnin AllNotInHandler) validateValueWithFloatPattern(_ float64, _ interface{}) bool {
	return false
}

func (allnin AllNotInHandler) validateValueWithMapPattern(_ map[string]interface{}, _ interface{}) bool {
	return false
}

func (allnin AllNotInHandler) validateValueWithSlicePattern(_ []interface{}, _ interface{}) bool {
	return false
}
