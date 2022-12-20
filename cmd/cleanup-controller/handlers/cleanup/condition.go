package cleanup

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
	"github.com/pkg/errors"
)

func checkAnyAllConditions(logger logr.Logger, ctx enginecontext.Interface, condition kyvernov2beta1.AnyAllConditions) (bool, error) {
	for _, condition := range condition.AllConditions {
		if passed, err := checkCondition(logger, ctx, condition); err != nil {
			return false, err
		} else if !passed {
			return false, nil
		}
	}
	for _, condition := range condition.AnyConditions {
		if passed, err := checkCondition(logger, ctx, condition); err != nil {
			return false, err
		} else if passed {
			return true, nil
		}
	}
	return len(condition.AnyConditions) == 0, nil
}

func checkCondition(logger logr.Logger, ctx enginecontext.Interface, condition kyvernov2beta1.Condition) (bool, error) {
	key, err := variables.SubstituteAllInPreconditions(logger, ctx, condition.GetKey())
	if err != nil {
		return false, errors.Wrapf(err, "failed to substitute variables in condition key")
	}
	value, err := variables.SubstituteAllInPreconditions(logger, ctx, condition.GetValue())
	if err != nil {
		return false, errors.Wrapf(err, "failed to substitute variables in condition value")
	}
	handler := operator.CreateOperatorHandler(logger, ctx, kyvernov1.ConditionOperator(condition.Operator))
	if handler == nil {
		return false, errors.Wrapf(err, "failed to create handler for condition operator")
	}
	return handler.Evaluate(key, value), nil
}
