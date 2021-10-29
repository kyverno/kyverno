package operator

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

//OperatorHandler provides interface to manage types
type OperatorHandler interface {
	Evaluate(key, value interface{}) bool
	validateValueWithStringPattern(key string, value interface{}) bool
	validateValueWithBoolPattern(key bool, value interface{}) bool
	validateValueWithIntPattern(key int64, value interface{}) bool
	validateValueWithFloatPattern(key float64, value interface{}) bool
	validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool
	validateValueWithSlicePattern(key []interface{}, value interface{}) bool
}

//VariableSubstitutionHandler defines the handler function for variable substitution
type VariableSubstitutionHandler = func(log logr.Logger, ctx context.EvalInterface, pattern interface{}) (interface{}, error)

//CreateOperatorHandler returns the operator handler based on the operator used in condition
func CreateOperatorHandler(log logr.Logger, ctx context.EvalInterface, op kyverno.ConditionOperator) OperatorHandler {
	str := strings.ToLower(string(op))
	switch str {

	case strings.ToLower(string(kyverno.Equal)),
		strings.ToLower(string(kyverno.Equals)):
		return NewEqualHandler(log, ctx)

	case strings.ToLower(string(kyverno.NotEqual)),
		strings.ToLower(string(kyverno.NotEquals)):
		return NewNotEqualHandler(log, ctx)

	// deprecated
	case strings.ToLower(string(kyverno.In)):
		return NewInHandler(log, ctx)

	case strings.ToLower(string(kyverno.AnyIn)):
		return NewAnyInHandler(log, ctx)

	case strings.ToLower(string(kyverno.AllIn)):
		return NewAllInHandler(log, ctx)

	// deprecated
	case strings.ToLower(string(kyverno.NotIn)):
		return NewNotInHandler(log, ctx)

	case strings.ToLower(string(kyverno.AnyNotIn)):
		return NewAnyNotInHandler(log, ctx)

	case strings.ToLower(string(kyverno.AllNotIn)):
		return NewAllNotInHandler(log, ctx)

	case strings.ToLower(string(kyverno.GreaterThanOrEquals)),
		strings.ToLower(string(kyverno.GreaterThan)),
		strings.ToLower(string(kyverno.LessThanOrEquals)),
		strings.ToLower(string(kyverno.LessThan)):
		return NewNumericOperatorHandler(log, ctx, op)

	case strings.ToLower(string(kyverno.DurationGreaterThanOrEquals)),
		strings.ToLower(string(kyverno.DurationGreaterThan)),
		strings.ToLower(string(kyverno.DurationLessThanOrEquals)),
		strings.ToLower(string(kyverno.DurationLessThan)):
		log.Info("DEPRECATED: The Duration* operators have been replaced with the other existing operators that now also support duration values", "operator", str)
		return NewDurationOperatorHandler(log, ctx, op)

	default:
		log.Info("operator not supported", "operator", str)
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
