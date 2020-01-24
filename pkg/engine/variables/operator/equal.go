package operator

import (
	"math"
	"reflect"
	"strconv"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/context"
)

//NewEqualHandler returns handler to manage Equal operations
func NewEqualHandler(ctx context.EvalInterface, subHandler VariableSubstitutionHandler) OperatorHandler {
	return EqualHandler{
		ctx:        ctx,
		subHandler: subHandler,
	}
}

//EqualHandler provides implementation to handle NotEqual Operator
type EqualHandler struct {
	ctx        context.EvalInterface
	subHandler VariableSubstitutionHandler
}

//Evaluate evaluates expression with Equal Operator
func (eh EqualHandler) Evaluate(key, value interface{}) bool {
	// substitute the variables
	nKey := eh.subHandler(eh.ctx, key)
	nValue := eh.subHandler(eh.ctx, value)
	// key and value need to be of same type
	switch typedKey := nKey.(type) {
	case bool:
		return eh.validateValuewithBoolPattern(typedKey, nValue)
	case int:
		return eh.validateValuewithIntPattern(int64(typedKey), nValue)
	case int64:
		return eh.validateValuewithIntPattern(typedKey, nValue)
	case float64:
		return eh.validateValuewithFloatPattern(typedKey, nValue)
	case string:
		return eh.validateValuewithStringPattern(typedKey, nValue)
	case map[string]interface{}:
		return eh.validateValueWithMapPattern(typedKey, nValue)
	case []interface{}:
		return eh.validateValueWithSlicePattern(typedKey, nValue)
	default:
		glog.Errorf("Unsupported type %v", typedKey)
		return false
	}
}

func (eh EqualHandler) validateValueWithSlicePattern(key []interface{}, value interface{}) bool {
	if val, ok := value.([]interface{}); ok {
		return reflect.DeepEqual(key, val)
	}
	glog.Warningf("Expected []interface{}, %v is of type %T", value, value)
	return false
}

func (eh EqualHandler) validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool {
	if val, ok := value.(map[string]interface{}); ok {
		return reflect.DeepEqual(key, val)
	}
	glog.Warningf("Expected map[string]interface{}, %v is of type %T", value, value)
	return false
}

func (eh EqualHandler) validateValuewithStringPattern(key string, value interface{}) bool {
	if val, ok := value.(string); ok {
		return key == val
	}
	glog.Warningf("Expected string, %v is of type %T", value, value)
	return false
}

func (eh EqualHandler) validateValuewithFloatPattern(key float64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		// check that float has not fraction
		if key == math.Trunc(key) {
			return int(key) == typedValue
		}
		glog.Warningf("Expected float, found int: %d\n", typedValue)
	case int64:
		// check that float has not fraction
		if key == math.Trunc(key) {
			return int64(key) == typedValue
		}
		glog.Warningf("Expected float, found int: %d\n", typedValue)
	case float64:
		return typedValue == key
	case string:
		// extract float from string
		float64Num, err := strconv.ParseFloat(typedValue, 64)
		if err != nil {
			glog.Warningf("Failed to parse float64 from string: %v", err)
			return false
		}
		return float64Num == key
	default:
		glog.Warningf("Expected float, found: %T\n", value)
		return false
	}
	return false
}

func (eh EqualHandler) validateValuewithBoolPattern(key bool, value interface{}) bool {
	typedValue, ok := value.(bool)
	if !ok {
		glog.Error("Expected bool, found %V", value)
		return false
	}
	return key == typedValue
}

func (eh EqualHandler) validateValuewithIntPattern(key int64, value interface{}) bool {
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
		glog.Warningf("Expected int, found float: %f", typedValue)
		return false
	case string:
		// extract in64 from string
		int64Num, err := strconv.ParseInt(typedValue, 10, 64)
		if err != nil {
			glog.Warningf("Failed to parse int64 from string: %v", err)
			return false
		}
		return int64Num == key
	default:
		glog.Warningf("Expected int, %v is of type %T", value, value)
		return false
	}
}
