package operator

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

// OperatorHandler provides interface to manage types
type OperatorHandler interface {
	Evaluate(key, value interface{}) bool
	validateValueWithStringPattern(key string, value interface{}) bool
	validateValueWithBoolPattern(key bool, value interface{}) bool
	validateValueWithIntPattern(key int64, value interface{}) bool
	validateValueWithFloatPattern(key float64, value interface{}) bool
	validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool
	validateValueWithSlicePattern(key []interface{}, value interface{}) bool
}

// VariableSubstitutionHandler defines the handler function for variable substitution
type VariableSubstitutionHandler = func(log logr.Logger, ctx context.EvalInterface, pattern interface{}) (interface{}, error)

// CreateOperatorHandler returns the operator handler based on the operator used in condition
func CreateOperatorHandler(log logr.Logger, ctx context.EvalInterface, op kyvernov1.ConditionOperator) OperatorHandler {
	str := strings.ToLower(string(op))
	switch str {
	case strings.ToLower(string(kyvernov1.ConditionOperators["Equal"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["Equals"])):
		return NewEqualHandler(log, ctx)

	case strings.ToLower(string(kyvernov1.ConditionOperators["NotEqual"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["NotEquals"])):
		return NewNotEqualHandler(log, ctx)

	// deprecated
	case strings.ToLower(string(kyvernov1.ConditionOperators["In"])):
		return NewInHandler(log, ctx)

	case strings.ToLower(string(kyvernov1.ConditionOperators["AnyIn"])):
		return NewAnyInHandler(log, ctx)

	case strings.ToLower(string(kyvernov1.ConditionOperators["AllIn"])):
		return NewAllInHandler(log, ctx)

	// deprecated
	case strings.ToLower(string(kyvernov1.ConditionOperators["NotIn"])):
		return NewNotInHandler(log, ctx)

	case strings.ToLower(string(kyvernov1.ConditionOperators["AnyNotIn"])):
		return NewAnyNotInHandler(log, ctx)

	case strings.ToLower(string(kyvernov1.ConditionOperators["AllNotIn"])):
		return NewAllNotInHandler(log, ctx)

	case strings.ToLower(string(kyvernov1.ConditionOperators["GreaterThanOrEquals"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["GreaterThan"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["LessThanOrEquals"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["LessThan"])):
		return NewNumericOperatorHandler(log, ctx, op)

	case strings.ToLower(string(kyvernov1.ConditionOperators["DurationGreaterThanOrEquals"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["DurationGreaterThan"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["DurationLessThanOrEquals"])),
		strings.ToLower(string(kyvernov1.ConditionOperators["DurationLessThan"])):
		log.V(2).Info("DEPRECATED: The Duration* operators have been replaced with the other existing operators that now also support duration values", "operator", str)
		return NewDurationOperatorHandler(log, ctx, op)

	default:
		log.V(2).Info("operator not supported", "operator", str)
	}

	return nil
}

func parseDuration(key, value interface{}) (*time.Duration, *time.Duration, error) {
	var keyDuration *time.Duration
	var valueDuration *time.Duration
	var err error

	// We need to first ensure at least one of the values is actually a duration string.
	switch typedKey := key.(type) {
	case string:
		duration, err := time.ParseDuration(typedKey)
		if err == nil && key != "0" {
			keyDuration = &duration
		}
	}
	switch typedValue := value.(type) {
	case string:
		duration, err := time.ParseDuration(typedValue)
		if err == nil && value != "0" {
			valueDuration = &duration
		}
	}
	if keyDuration == nil && valueDuration == nil {
		return keyDuration, valueDuration, fmt.Errorf("neither value is a duration")
	}

	if keyDuration == nil {
		var duration time.Duration

		switch typedKey := key.(type) {
		case int:
			duration = time.Duration(typedKey) * time.Second
		case int64:
			duration = time.Duration(typedKey) * time.Second
		case float64:
			duration = time.Duration(typedKey) * time.Second
		default:
			return keyDuration, valueDuration, fmt.Errorf("no valid duration value")
		}

		keyDuration = &duration
	}

	if valueDuration == nil {
		var duration time.Duration

		switch typedValue := value.(type) {
		case int:
			duration = time.Duration(typedValue) * time.Second
		case int64:
			duration = time.Duration(typedValue) * time.Second
		case float64:
			duration = time.Duration(typedValue) * time.Second
		default:
			return keyDuration, valueDuration, fmt.Errorf("no valid duration value")
		}

		valueDuration = &duration
	}

	return keyDuration, valueDuration, err
}
