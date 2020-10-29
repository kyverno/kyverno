package operator

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"strings"
)

//OperatorHandler provides interface to manage types
type OperatorHandler interface {
	Evaluate(key, value interface{}) bool
	validateValuewithBoolPattern(key bool, value interface{}) bool
	validateValuewithIntPattern(key int64, value interface{}) bool
	validateValuewithFloatPattern(key float64, value interface{}) bool
	validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool
	validateValueWithSlicePattern(key []interface{}, value interface{}) bool
}

//VariableSubstitutionHandler defines the handler function for variable substitution
type VariableSubstitutionHandler = func(log logr.Logger, ctx context.EvalInterface, pattern interface{}) (interface{}, error)

//CreateOperatorHandler returns the operator handler based on the operator used in condition
func CreateOperatorHandler(log logr.Logger, ctx context.EvalInterface, op kyverno.ConditionOperator, subHandler VariableSubstitutionHandler) OperatorHandler {
	str := strings.ToLower(string(op))
	switch str {

	case strings.ToLower(string(kyverno.Equal)):
		return NewEqualHandler(log, ctx, subHandler)

	case strings.ToLower(string(kyverno.Equals)):
		return NewEqualHandler(log, ctx, subHandler)

	case strings.ToLower(string(kyverno.NotEqual)):
		return NewNotEqualHandler(log, ctx, subHandler)

	case strings.ToLower(string(kyverno.NotEquals)):
		return NewNotEqualHandler(log, ctx, subHandler)

	case strings.ToLower(string(kyverno.In)):
		return NewInHandler(log, ctx, subHandler)

	case strings.ToLower(string(kyverno.NotIn)):
		return NewNotInHandler(log, ctx, subHandler)

	default:
		log.Info("operator not supported", "operator", str)
	}

	return nil
}
