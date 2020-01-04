package operator

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
)

type OperatorHandler interface {
	Evaluate(key, value interface{}) bool
	validateValuewithBoolPattern(key bool, value interface{}) bool
	validateValuewithIntPattern(key int64, value interface{}) bool
	validateValuewithFloatPattern(key float64, value interface{}) bool
	validateValueWithMapPattern(key map[string]interface{}, value interface{}) bool
	validateValueWithSicePattern(key []interface{}, value interface{}) bool
}

type VariableSubstitutionHandler = func(ctx context.EvalInterface, pattern interface{}) interface{}

func CreateOperatorHandler(ctx context.EvalInterface, op kyverno.ConditionOperator, subHandler VariableSubstitutionHandler) OperatorHandler {
	switch op {
	case kyverno.Equal:
		return NewEqualHandler(ctx, subHandler)
	case kyverno.NotEqual:
		return NewNotEqualHandler(ctx, subHandler)
	case kyverno.In:
	case kyverno.NotIn:
	default:
		glog.Errorf("unsupported operator: %s", string(op))
	}
	return nil
}
