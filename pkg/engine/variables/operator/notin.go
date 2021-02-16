package operator

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
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
		return nin.validateValueWithStringPattern(typedKey, value)
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
	invalidType, keyExists := setExistsInArray(key, value, nin.log, true)
	if invalidType {
		nin.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return !keyExists
}

// checkInSubsetForNotIn checks if ANY of the values of S1 is in S2
func checkInSubsetForNotIn(key []string, value []string) bool {
	set := make(map[string]int)

	for _, val := range value {
		set[val]++
	}

	for _, val := range key {
		_, found := set[val]
		if found {
			return true
		}
	}

	return false
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
