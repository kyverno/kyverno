package operator

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/engine/context"
)

//NewNotInHandler returns handler to manage NotIn operations
func NewNotInHandler(log logr.Logger, ctx context.EvalInterface, subHandler VariableSubstitutionHandler) OperatorHandler {
	return NotInHandler{
		ctx:        ctx,
		subHandler: subHandler,
		log:        log,
	}
}

//NotInHandler provides implementation to handle NotIn Operator
type NotInHandler struct {
	ctx        context.EvalInterface
	subHandler VariableSubstitutionHandler
	log        logr.Logger
}

//Evaluate evaluates expression with NotIn Operator
func (nin NotInHandler) Evaluate(key, value interface{}) bool {
	var err error
	// substitute the variables
	if key, err = nin.subHandler(nin.log, nin.ctx, key); err != nil {
		nin.log.Error(err, "Failed to resolve variable", "variable", key)
		return false
	}
	if value, err = nin.subHandler(nin.log, nin.ctx, value); err != nil {
		nin.log.Error(err, "Failed to resolve variable", "variable", value)
		return false
	}

	switch typedKey := key.(type) {
	case string:
		return nin.validateValuewithStringPattern(typedKey, value)
	default:
		nin.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}

}

func (nin NotInHandler) validateValuewithStringPattern(key string, value interface{}) bool {
	invalidType, keyExists := ValidateStringPattern(key, value)
	if invalidType {
		nin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	if !keyExists {
		return true
	}

	return false
}

func (nin NotInHandler) validateValuewithBoolPattern(key bool, value interface{}) bool {
	return false
}

func (nin NotInHandler) validateValuewithIntPattern(key int64, value interface{}) bool {
	return false
}

func (nin NotInHandler) validateValuewithFloatPattern(key float64, value interface{}) bool {
	return false
}

func (nin NotInHandler) validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool {
	return false
}

func (nin NotInHandler) validateValueWithSlicePattern(key []interface{}, value interface{}) bool {
	return false
}
