package operator

import (
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
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
		return NewDurationOperatorHandler(log, ctx, op)

	default:
		log.Info("operator not supported", "operator", str)
	}

	return nil
}
