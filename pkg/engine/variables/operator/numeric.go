package operator

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewNumericOperatorHandler returns handler to manage the provided numeric operations (>, >=, <=, <)
func NewNumericOperatorHandler(log logr.Logger, ctx context.EvalInterface, subHandler VariableSubstitutionHandler, op kyverno.ConditionOperator) OperatorHandler {
	return NumericOperatorHandler{
		ctx:        ctx,
		subHandler: subHandler,
		log:        log,
		condition:  op,
	}
}

//NumericOperatorHandler provides implementation to handle Numeric Operations associated with policies
type NumericOperatorHandler struct {
	ctx        context.EvalInterface
	subHandler VariableSubstitutionHandler
	log        logr.Logger
	condition  kyverno.ConditionOperator
}

// compareByCondition compares a float64 key with a float64 value on the basis of the provided operator
func compareByCondition(key float64, value float64, op kyverno.ConditionOperator, log *logr.Logger) bool {
	switch op {
	case kyverno.GreaterThanOrEquals:
		return key >= value
	case kyverno.GreaterThan:
		return key > value
	case kyverno.LessThanOrEquals:
		return key <= value
	case kyverno.LessThan:
		return key < value
	default:
		(*log).Info(fmt.Sprintf("Expected operator, one of [GreaterThanOrEquals, GreaterThan, LessThanOrEquals, LessThan], found %s", op))
		return false
	}
}

func (noh NumericOperatorHandler) Evaluate(key, value interface{}, isPreCondition bool) bool {
	var err error
	if key, err = noh.subHandler(noh.log, noh.ctx, key); err != nil {
		// Failed to resolve the variable
		if isPreCondition {
			noh.log.Info("Failed to resolve variable", "info", err.Error(), "variable", key)
		} else {
			noh.log.Error(err, "Failed to resolve variable", "variable", key)
		}
		return false
	}
	if value, err = noh.subHandler(noh.log, noh.ctx, value); err != nil {
		// Failed to resolve the variable
		if isPreCondition {
			noh.log.Info("Failed to resolve variable", "info", err.Error(), "variable", value)
		} else {
			noh.log.Error(err, "Failed to resolve variable", "variable", value)
		}
		return false
	}

	switch typedKey := key.(type) {
	case int:
		return noh.validateValueWithIntPattern(int64(typedKey), value)
	case int64:
		return noh.validateValueWithIntPattern(typedKey, value)
	case float64:
		return noh.validateValueWithFloatPattern(typedKey, value)
	case string:
		return noh.validateValueWithStringPattern(typedKey, value)
	default:
		noh.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithIntPattern(key int64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return compareByCondition(float64(key), float64(typedValue), noh.condition, &noh.log)
	case int64:
		return compareByCondition(float64(key), float64(typedValue), noh.condition, &noh.log)
	case float64:
		return compareByCondition(float64(key), typedValue, noh.condition, &noh.log)
	case string:
		// extract float64 and (if that fails) then, int64 from the string
		float64val, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			return compareByCondition(float64(key), float64val, noh.condition, &noh.log)
		}
		int64val, err := strconv.ParseInt(typedValue, 10, 64)
		if err == nil {
			return compareByCondition(float64(key), float64(int64val), noh.condition, &noh.log)
		}
		noh.log.Error(fmt.Errorf("Parse Error: "), "Failed to parse both float64 and int64 from the string value")
		return false
	default:
		noh.log.Info("Expected type int", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithFloatPattern(key float64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return compareByCondition(key, float64(typedValue), noh.condition, &noh.log)
	case int64:
		return compareByCondition(key, float64(typedValue), noh.condition, &noh.log)
	case float64:
		return compareByCondition(key, typedValue, noh.condition, &noh.log)
	case string:
		float64val, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			return compareByCondition(key, float64val, noh.condition, &noh.log)
		}
		int64val, err := strconv.ParseInt(typedValue, 10, 64)
		if err == nil {
			return compareByCondition(key, float64(int64val), noh.condition, &noh.log)
		}
		noh.log.Error(fmt.Errorf("Parse Error: "), "Failed to parse both float64 and int64 from the string value")
		return false
	default:
		noh.log.Info("Expected type float", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	// extracting float64 from the string key
	float64key, err := strconv.ParseFloat(key, 64)
	if err == nil {
		return noh.validateValueWithFloatPattern(float64key, value)
	}
	// extracting int64 from the string because float64 extraction failed
	int64key, err := strconv.ParseInt(key, 10, 64)
	if err == nil {
		return noh.validateValueWithIntPattern(int64key, value)
	}
	noh.log.Error(err, "Failed to parse both float64 and int64 from the string keyt")
	return false
}

// the following functions are unreachable because the key is strictly supposed to be numeric
// still the following functions are just created to make NumericOperatorHandler struct implement OperatorHandler interface
func (noh NumericOperatorHandler) validateValueWithBoolPattern(key bool, value interface{}) bool {
	return false
}
func (noh NumericOperatorHandler) validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool {
	return false
}
func (noh NumericOperatorHandler) validateValueWithSlicePattern(key []interface{}, value interface{}) bool {
	return false
}
