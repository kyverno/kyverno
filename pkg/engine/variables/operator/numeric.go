package operator

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/api/resource"
)

//NewNumericOperatorHandler returns handler to manage the provided numeric operations (>, >=, <=, <)
func NewNumericOperatorHandler(log logr.Logger, ctx context.EvalInterface, op kyverno.ConditionOperator) OperatorHandler {
	return NumericOperatorHandler{
		ctx:       ctx,
		log:       log,
		condition: op,
	}
}

//NumericOperatorHandler provides implementation to handle Numeric Operations associated with policies
type NumericOperatorHandler struct {
	ctx       context.EvalInterface
	log       logr.Logger
	condition kyverno.ConditionOperator
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
	case kyverno.Equals:
		return key == value
	case kyverno.Equal:
		return key == value
	case kyverno.NotEquals:
		return key != value
	case kyverno.NotEqual:
		return key != value
	default:
		(*log).Info(fmt.Sprintf("Expected operator, one of [GreaterThanOrEquals, GreaterThan, LessThanOrEquals, LessThan, Equals, NotEquals], found %s", op))
		return false
	}
}

func (noh NumericOperatorHandler) Evaluate(key, value interface{}) bool {
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
		durationKey, durationValue, err := parseDuration(key, value)
		if err == nil {
			return compareByCondition(float64(durationKey.Seconds()), float64(durationValue.Seconds()), noh.condition, &noh.log)
		}
		// extract float64 and (if that fails) then, int64 from the string
		float64val, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			return compareByCondition(float64(key), float64val, noh.condition, &noh.log)
		}
		int64val, err := strconv.ParseInt(typedValue, 10, 64)
		if err == nil {
			return compareByCondition(float64(key), float64(int64val), noh.condition, &noh.log)
		}
		noh.log.Error(fmt.Errorf("parse error: "), "Failed to parse both float64 and int64 from the string value")
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
		durationKey, durationValue, err := parseDuration(key, value)
		if err == nil {
			return compareByCondition(float64(durationKey.Seconds()), float64(durationValue.Seconds()), noh.condition, &noh.log)
		}
		float64val, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			return compareByCondition(key, float64val, noh.condition, &noh.log)
		}
		int64val, err := strconv.ParseInt(typedValue, 10, 64)
		if err == nil {
			return compareByCondition(key, float64(int64val), noh.condition, &noh.log)
		}
		noh.log.Error(fmt.Errorf("parse error: "), "Failed to parse both float64 and int64 from the string value")
		return false
	default:
		noh.log.Info("Expected type float", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithResourcePattern(key resource.Quantity, value interface{}) bool {
	switch typedValue := value.(type) {
	case string:
		resourceValue, err := resource.ParseQuantity(typedValue)
		if err != nil {
			noh.log.Error(fmt.Errorf("parse error: "), "Failed to parse value type doesn't match key type")
			return false
		}
		return compareByCondition(float64(key.Cmp(resourceValue)), 0, noh.condition, &noh.log)
	default:
		noh.log.Info("Expected type string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	// We need to check duration first as it's the only type that can be compared to a different type
	durationKey, durationValue, err := parseDuration(key, value)
	if err == nil {
		return compareByCondition(float64(durationKey.Seconds()), float64(durationValue.Seconds()), noh.condition, &noh.log)
	}
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
	// attempt to extract resource quantity from string
	resourceKey, err := resource.ParseQuantity(key)
	if err == nil {
		return noh.validateValueWithResourcePattern(resourceKey, value)
	}

	noh.log.Error(err, "Failed to parse from the string key, value is not float, int nor resource quantity")
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
