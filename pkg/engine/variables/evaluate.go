package variables

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
)

// Evaluate evaluates the condition
func Evaluate(logger logr.Logger, ctx context.EvalInterface, condition kyvernov1.Condition) (bool, string, error) {
	key, err := SubstituteAllInPreconditions(logger, ctx, condition.GetKey())
	if err != nil {
		return false, "", fmt.Errorf("failed to substitute variables in condition key: %w", err)
	}
	value, err := SubstituteAllInPreconditions(logger, ctx, condition.GetValue())
	if err != nil {
		return false, "", fmt.Errorf("failed to substitute variables in condition value: %w", err)
	}
	handler := operator.CreateOperatorHandler(logger, ctx, condition.Operator)
	if handler == nil {
		return false, "", fmt.Errorf("failed to create handler for condition operator: %w", err)
	}
	return handler.Evaluate(key, value), condition.Message, nil
}

// EvaluateConditions evaluates all the conditions present in a slice, in a backwards compatible way
func EvaluateConditions(log logr.Logger, ctx context.EvalInterface, conditions interface{}) (bool, string, error) {
	return EvaluateConditionsWithContext(log, ctx, conditions, "")
}

func EvaluateConditionsWithContext(log logr.Logger, ctx context.EvalInterface, conditions interface{}, conditionContext string) (bool, string, error) {
	switch typedConditions := conditions.(type) {
	case *kyvernov1.AnyAllConditions:
		return evaluateAnyAllConditionsWithContext(log, ctx, *typedConditions, conditionContext)
	case kyvernov1.AnyAllConditions:
		return evaluateAnyAllConditionsWithContext(log, ctx, typedConditions, conditionContext)
	case []kyvernov1.Condition: // backwards compatibility
		return evaluateOldConditions(log, ctx, typedConditions)
	}
	return false, "", fmt.Errorf("invalid condition")
}

func EvaluateAnyAllConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.AnyAllConditions) (bool, string, error) {
	return EvaluateAnyAllConditionsWithContext(log, ctx, conditions, "")
}

func EvaluateAnyAllConditionsWithContext(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.AnyAllConditions, conditionContext string) (bool, string, error) {
	if conditionContext == "" {
		conditionContext = "condition"
	}
	var conditionTrueMessages []string
	for i, c := range conditions {
		if val, msg, err := evaluateAnyAllConditionsWithContext(log, ctx, c, fmt.Sprintf("%s[%d]", conditionContext, i)); err != nil {
			return false, "", err
		} else if !val {
			return false, msg, nil
		} else {
			conditionTrueMessages = append(conditionTrueMessages, msg)
		}
	}

	return true, stringutils.JoinNonEmpty(conditionTrueMessages, ";"), nil
}

func evaluateAnyAllConditionsWithContext(log logr.Logger, ctx context.EvalInterface, conditions kyvernov1.AnyAllConditions, conditionContext string) (bool, string, error) {
	anyConditions, allConditions := conditions.AnyConditions, conditions.AllConditions
	anyConditionsResult, allConditionsResult := true, true
	var conditionFalseMessages []string
	var conditionTrueMessages []string

	// update the anyConditionsResult if they are present
	if anyConditions != nil {
		anyConditionsResult = false
		for _, condition := range anyConditions {
			if val, msg, err := Evaluate(log, ctx, condition); err != nil {
				return false, "", err
			} else if val {
				anyConditionsResult = true
				conditionTrueMessages = append(conditionTrueMessages, msg)
				break
			} else {
				conditionFalseMessages = append(conditionFalseMessages, msg)
			}
		}

		if !anyConditionsResult {
			log.V(3).Info("no condition passed for 'any' block", "any", anyConditions, "context", conditionContext)
		}
	}

	// update the allConditionsResult if they are present
	for idx, condition := range allConditions {
		if val, msg, err := Evaluate(log, ctx, condition); err != nil {
			return false, "", err
		} else if !val {
			allConditionsResult = false
			conditionFalseMessages = append(conditionFalseMessages, msg)
			log.V(3).Info("a condition failed in 'all' block", "condition", condition, "message", msg, "index", idx, "context", conditionContext)
			break
		} else {
			conditionTrueMessages = append(conditionTrueMessages, msg)
		}
	}

	finalResult := anyConditionsResult && allConditionsResult
	if finalResult {
		return finalResult, stringutils.JoinNonEmpty(conditionTrueMessages, "; "), nil
	}

	return finalResult, stringutils.JoinNonEmpty(conditionFalseMessages, "; "), nil
}

// evaluateOldConditions evaluates multiple conditions when those conditions are provided in the old manner i.e. without 'any' or 'all'
func evaluateOldConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.Condition) (bool, string, error) {
	var conditionTrueMessages []string
	for _, condition := range conditions {
		if val, msg, err := Evaluate(log, ctx, condition); err != nil {
			return false, "", err
		} else if !val {
			return false, msg, nil
		} else {
			conditionTrueMessages = append(conditionTrueMessages, msg)
		}
	}

	return true, stringutils.JoinNonEmpty(conditionTrueMessages, ";"), nil
}
