package operator

import (
	"fmt"
	"strconv"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/api/resource"
)

// NewNumericOperatorHandler returns handler to manage the provided numeric operations (>, >=, <=, <)
func NewNumericOperatorHandler(log logr.Logger, ctx context.EvalInterface, op kyvernov1.ConditionOperator) OperatorHandler {
	return NumericOperatorHandler{
		ctx:       ctx,
		log:       log,
		condition: op,
	}
}

// NumericOperatorHandler provides implementation to handle Numeric Operations associated with policies
type NumericOperatorHandler struct {
	ctx       context.EvalInterface
	log       logr.Logger
	condition kyvernov1.ConditionOperator
}

// compareByCondition compares a float64 key with a float64 value on the basis of the provided operator
func compareByCondition(key float64, value float64, op kyvernov1.ConditionOperator, log logr.Logger) bool {
	switch op {
	case kyvernov1.ConditionOperators["GreaterThanOrEquals"]:
		return key >= value
	case kyvernov1.ConditionOperators["GreaterThan"]:
		return key > value
	case kyvernov1.ConditionOperators["LessThanOrEquals"]:
		return key <= value
	case kyvernov1.ConditionOperators["LessThan"]:
		return key < value
	default:
		log.V(2).Info(fmt.Sprintf("Expected operator, one of [GreaterThanOrEquals, GreaterThan, LessThanOrEquals, LessThan, Equals, NotEquals], found %s", op))
		return false
	}
}

func compareVersionByCondition(key semver.Version, value semver.Version, op kyvernov1.ConditionOperator, log logr.Logger) bool {
	switch op {
	case kyvernov1.ConditionOperators["GreaterThanOrEquals"]:
		return key.GTE(value)
	case kyvernov1.ConditionOperators["GreaterThan"]:
		return key.GT(value)
	case kyvernov1.ConditionOperators["LessThanOrEquals"]:
		return key.LTE(value)
	case kyvernov1.ConditionOperators["LessThan"]:
		return key.LT(value)
	default:
		log.V(2).Info(fmt.Sprintf("Expected operator, one of [GreaterThanOrEquals, GreaterThan, LessThanOrEquals, LessThan, Equals, NotEquals], found %s", op))
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
		noh.log.V(2).Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithIntPattern(key int64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return compareByCondition(float64(key), float64(typedValue), noh.condition, noh.log)
	case int64:
		return compareByCondition(float64(key), float64(typedValue), noh.condition, noh.log)
	case float64:
		return compareByCondition(float64(key), typedValue, noh.condition, noh.log)
	case string:
		durationKey, durationValue, err := parseDuration(key, value)
		if err == nil {
			return compareByCondition(durationKey.Seconds(), durationValue.Seconds(), noh.condition, noh.log)
		}
		// extract float64 and (if that fails) then, int64 from the string
		float64val, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			return compareByCondition(float64(key), float64val, noh.condition, noh.log)
		}
		int64val, err := strconv.ParseInt(typedValue, 10, 64)
		if err == nil {
			return compareByCondition(float64(key), float64(int64val), noh.condition, noh.log)
		}
		noh.log.Error(fmt.Errorf("parse error: "), "Failed to parse both float64 and int64 from the string value")
		return false
	default:
		noh.log.V(2).Info("Expected type int", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithFloatPattern(key float64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return compareByCondition(key, float64(typedValue), noh.condition, noh.log)
	case int64:
		return compareByCondition(key, float64(typedValue), noh.condition, noh.log)
	case float64:
		return compareByCondition(key, typedValue, noh.condition, noh.log)
	case string:
		durationKey, durationValue, err := parseDuration(key, value)
		if err == nil {
			return compareByCondition(durationKey.Seconds(), durationValue.Seconds(), noh.condition, noh.log)
		}
		float64val, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			return compareByCondition(key, float64val, noh.condition, noh.log)
		}
		int64val, err := strconv.ParseInt(typedValue, 10, 64)
		if err == nil {
			return compareByCondition(key, float64(int64val), noh.condition, noh.log)
		}
		noh.log.Error(fmt.Errorf("parse error: "), "Failed to parse both float64 and int64 from the string value")
		return false
	default:
		noh.log.V(2).Info("Expected type float", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithVersionPattern(key semver.Version, value interface{}) bool {
	switch typedValue := value.(type) {
	case string:
		versionValue, err := semver.Parse(typedValue)
		if err != nil {
			noh.log.Error(fmt.Errorf("parse error: "), "Failed to parse value type doesn't match key type")
			return false
		}
		return compareVersionByCondition(key, versionValue, noh.condition, noh.log)
	default:
		noh.log.V(2).Info("Expected type string", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (noh NumericOperatorHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	// We need to check duration first as it's the only type that can be compared to a different type
	durationKey, durationValue, err := parseDuration(key, value)
	if err == nil {
		return compareByCondition(durationKey.Seconds(), durationValue.Seconds(), noh.condition, noh.log)
	}
	// attempt to extract resource quantity from string before parsing floats/ints as resources can also be ints/floats represented as string type
	resourceKey, resourceValue, err := parseQuantity(key, value)
	if err == nil {
		return compareByCondition(float64(resourceKey.Cmp(resourceValue)), 0, noh.condition, noh.log)
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
	// attempt to extract version from string
	versionKey, err := semver.Parse(key)
	if err == nil {
		return noh.validateValueWithVersionPattern(versionKey, value)
	}

	noh.log.Error(err, "Failed to parse from the string key, value is not float, int nor resource quantity")
	return false
}

func parseQuantity(key, value interface{}) (parsedKey, parsedValue resource.Quantity, err error) {
	switch typedKey := key.(type) {
	case string:
		parsedKey, err = resource.ParseQuantity(typedKey)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("key is not a quantity")
		return
	}
	switch typedValue := value.(type) {
	case string:
		parsedValue, err = resource.ParseQuantity(typedValue)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("value is not a quantity")
		return
	}
	return
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
