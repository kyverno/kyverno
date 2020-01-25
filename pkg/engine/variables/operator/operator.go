package operator

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
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
type VariableSubstitutionHandler = func(ctx context.EvalInterface, pattern interface{}) interface{}

//CreateOperatorHandler returns the operator handler based on the operator used in condition
func CreateOperatorHandler(ctx context.EvalInterface, op kyverno.ConditionOperator, subHandler VariableSubstitutionHandler) OperatorHandler {
	switch op {
	case kyverno.Equal:
		return NewEqualHandler(ctx, subHandler)
	case kyverno.NotEqual:
		return NewNotEqualHandler(ctx, subHandler)
	default:
		glog.Errorf("unsupported operator: %s", string(op))
	}
	return nil
}
