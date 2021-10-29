package operator

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//NewDurationOperatorHandler returns handler to manage the provided duration operations (>, >=, <=, <)
func NewDurationOperatorHandler(log logr.Logger, ctx context.EvalInterface, op kyverno.ConditionOperator) OperatorHandler {
	return DurationOperatorHandler{
		ctx:       ctx,
		log:       log,
		condition: op,
	}
}

//DurationOperatorHandler provides implementation to handle Duration Operations associated with policies
type DurationOperatorHandler struct {
	ctx       context.EvalInterface
	log       logr.Logger
	condition kyverno.ConditionOperator
}

// durationCompareByCondition compares a time.Duration key with a time.Duration value on the basis of the provided operator
func durationCompareByCondition(key time.Duration, value time.Duration, op kyverno.ConditionOperator, log *logr.Logger) bool {
	switch op {
	case kyverno.DurationGreaterThanOrEquals:
		return key >= value
	case kyverno.DurationGreaterThan:
		return key > value
	case kyverno.DurationLessThanOrEquals:
		return key <= value
	case kyverno.DurationLessThan:
		return key < value
	default:
		(*log).Info(fmt.Sprintf("Expected operator, one of [DurationGreaterThanOrEquals, DurationGreaterThan, DurationLessThanOrEquals, DurationLessThan], found %s", op))
		return false
	}
}

func (doh DurationOperatorHandler) Evaluate(key, value interface{}) bool {
	switch typedKey := key.(type) {
	case int:
		return doh.validateValueWithIntPattern(int64(typedKey), value)
	case int64:
		return doh.validateValueWithIntPattern(typedKey, value)
	case float64:
		return doh.validateValueWithFloatPattern(typedKey, value)
	case string:
		return doh.validateValueWithStringPattern(typedKey, value)
	default:
		doh.log.Info("Unsupported type", "value", typedKey, "type", fmt.Sprintf("%T", typedKey))
		return false
	}
}

func (doh DurationOperatorHandler) validateValueWithIntPattern(key int64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return durationCompareByCondition(time.Duration(key)*time.Second, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case int64:
		return durationCompareByCondition(time.Duration(key)*time.Second, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case float64:
		return durationCompareByCondition(time.Duration(key)*time.Second, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case string:
		duration, err := time.ParseDuration(typedValue)
		if err == nil {
			return durationCompareByCondition(time.Duration(key)*time.Second, duration, doh.condition, &doh.log)
		}
		doh.log.Error(fmt.Errorf("parse error: "), "Failed to parse time duration from the string value")
		return false
	default:
		doh.log.Info("Unexpected type", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (doh DurationOperatorHandler) validateValueWithFloatPattern(key float64, value interface{}) bool {
	switch typedValue := value.(type) {
	case int:
		return durationCompareByCondition(time.Duration(key)*time.Second, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case int64:
		return durationCompareByCondition(time.Duration(key)*time.Second, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case float64:
		return durationCompareByCondition(time.Duration(key)*time.Second, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case string:
		duration, err := time.ParseDuration(typedValue)
		if err == nil {
			return durationCompareByCondition(time.Duration(key)*time.Second, duration, doh.condition, &doh.log)
		}
		doh.log.Error(fmt.Errorf("parse error: "), "Failed to parse time duration from the string value")
		return false
	default:
		doh.log.Info("Unexpected type", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

func (doh DurationOperatorHandler) validateValueWithStringPattern(key string, value interface{}) bool {
	duration, err := time.ParseDuration(key)
	if err != nil {
		doh.log.Error(err, "Failed to parse time duration from the string key")
		return false
	}
	switch typedValue := value.(type) {
	case int:
		return durationCompareByCondition(duration, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case int64:
		return durationCompareByCondition(duration, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case float64:
		return durationCompareByCondition(duration, time.Duration(typedValue)*time.Second, doh.condition, &doh.log)
	case string:
		durationValue, err := time.ParseDuration(typedValue)
		if err == nil {
			return durationCompareByCondition(duration, durationValue, doh.condition, &doh.log)
		}
		doh.log.Error(fmt.Errorf("parse error: "), "Failed to parse time duration from the string value")
		return false
	default:
		doh.log.Info("Unexpected type", "value", value, "type", fmt.Sprintf("%T", value))
		return false
	}
}

// the following functions are unreachable because the key is strictly supposed to be a duration
// still the following functions are just created to make DurationOperatorHandler struct implement OperatorHandler interface
func (doh DurationOperatorHandler) validateValueWithBoolPattern(key bool, value interface{}) bool {
	return false
}
func (doh DurationOperatorHandler) validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool {
	return false
}
func (doh DurationOperatorHandler) validateValueWithSlicePattern(key []interface{}, value interface{}) bool {
	return false
}
