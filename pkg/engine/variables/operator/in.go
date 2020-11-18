package operator

import (
	"encoding/json"
	"fmt"
	"reflect"

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
		return in.validateValuewithStringPattern(typedKey, value)
	default:
		in.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (in InHandler) validateValuewithStringPattern(key string, value interface{}) (keyExists bool) {
	invalidType, keyExists := ValidateStringPattern(key, value, in.log)
	if invalidType {
		in.log.Info("expected type []string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}

	return keyExists
}

// ValidateStringPattern ...
func ValidateStringPattern(key string, value interface{}, log logr.Logger) (invalidType bool, keyExists bool) {
	stringType := reflect.TypeOf("")
	switch valuesAvaliable := value.(type) {
	case []interface{}:
		for _, val := range valuesAvaliable {
			if reflect.TypeOf(val) != stringType {
				return true, false
			}
			if key == val {
				keyExists = true
			}
		}

	// add to handle the configMap lookup, as configmap.data
	// takes string-string map, when looking for a value of array
	// data:
	//   key: "[\"value1\", \"value2\"]"
	// it will first unmarshal it to string slice, then compare
	case string:
		var arr []string
		if err := json.Unmarshal([]byte(valuesAvaliable), &arr); err != nil {
			log.Error(err, "failed to unmarshal to string slice", "value", value)
			return invalidType, keyExists
		}

		for _, val := range arr {
			if key == val {
				keyExists = true
			}
		}
	default:
		return true, false
	}

	return invalidType, keyExists
}

func (in InHandler) validateValuewithBoolPattern(key bool, value interface{}) bool {
	return false
}

func (in InHandler) validateValuewithIntPattern(key int64, value interface{}) bool {
	return false
}

func (in InHandler) validateValuewithFloatPattern(key float64, value interface{}) bool {
	return false
}

func (in InHandler) validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool {
	return false
}

func (in InHandler) validateValueWithSlicePattern(key []interface{}, value interface{}) bool {
	return false
}
