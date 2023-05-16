package variables

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
)

// Evaluate evaluates the condition
func Evaluate(log logr.Logger, ctx context.EvalInterface, condition kyvernov1.Condition) (bool, string) {
	// get handler for the operator
	handle := operator.CreateOperatorHandler(log, ctx, condition.Operator)
	if handle == nil {
		return false, condition.Message
	}

	return handle.Evaluate(condition.GetKey(), condition.GetValue()), condition.Message
}

// EvaluateConditions evaluates all the conditions present in a slice, in a backwards compatible way
func EvaluateConditions(log logr.Logger, ctx context.EvalInterface, conditions interface{}) (bool, string) {
	switch typedConditions := conditions.(type) {
	case kyvernov1.AnyAllConditions:
		return evaluateAnyAllConditions(log, ctx, typedConditions)
	case []kyvernov1.Condition: // backwards compatibility
		return evaluateOldConditions(log, ctx, typedConditions)
	}
	return false, "invalid condition"
}

func EvaluateAnyAllConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.AnyAllConditions) (bool, string) {
	var conditionTrueMessages []string
	for _, c := range conditions {
		if val, msg := evaluateAnyAllConditions(log, ctx, c); !val {
			return false, msg
		} else {
			conditionTrueMessages = append(conditionTrueMessages, msg)
		}
	}

	return true, stringutils.JoinNonEmpty(conditionTrueMessages, ";")
}

// evaluateAnyAllConditions evaluates multiple conditions as a logical AND (all) or OR (any) operation depending on the conditions
func evaluateAnyAllConditions(log logr.Logger, ctx context.EvalInterface, conditions kyvernov1.AnyAllConditions) (bool, string) {
	anyConditions, allConditions := conditions.AnyConditions, conditions.AllConditions
	anyConditionsResult, allConditionsResult := true, true
	var conditionFalseMessages []string
	var conditionTrueMessages []string

	// update the anyConditionsResult if they are present
	if anyConditions != nil {
		anyConditionsResult = false
		for _, condition := range anyConditions {
			if val, msg := Evaluate(log, ctx, condition); val {
				anyConditionsResult = true
				conditionTrueMessages = append(conditionTrueMessages, msg)
				break
			} else {
				conditionFalseMessages = append(conditionFalseMessages, msg)
			}
		}

		if !anyConditionsResult {
			log.V(3).Info("no condition passed for 'any' block", "any", anyConditions)
		}
	}

	// update the allConditionsResult if they are present
	for _, condition := range allConditions {
		if val, msg := Evaluate(log, ctx, condition); !val {
			allConditionsResult = false
			conditionFalseMessages = append(conditionFalseMessages, msg)
			log.V(3).Info("a condition failed in 'all' block", "condition", condition, "message", msg)
			break
		} else {
			conditionTrueMessages = append(conditionTrueMessages, msg)
		}
	}

	finalResult := anyConditionsResult && allConditionsResult
	if finalResult {
		return finalResult, stringutils.JoinNonEmpty(conditionTrueMessages, "; ")
	}

	return finalResult, stringutils.JoinNonEmpty(conditionFalseMessages, "; ")
}

// evaluateOldConditions evaluates multiple conditions when those conditions are provided in the old manner i.e. without 'any' or 'all'
func evaluateOldConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.Condition) (bool, string) {
	var conditionTrueMessages []string
	for _, condition := range conditions {
		if val, msg := Evaluate(log, ctx, condition); !val {
			return false, msg
		} else {
			conditionTrueMessages = append(conditionTrueMessages, msg)
		}
	}

	return true, stringutils.JoinNonEmpty(conditionTrueMessages, ";")
}
