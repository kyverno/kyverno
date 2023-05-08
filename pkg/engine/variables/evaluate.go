package variables

import (
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
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
	for _, c := range conditions {
		if val, msg := evaluateAnyAllConditions(log, ctx, c); !val {
			return false, msg
		}
	}

	return true, ""
}

// evaluateAnyAllConditions evaluates multiple conditions as a logical AND (all) or OR (any) operation depending on the conditions
func evaluateAnyAllConditions(log logr.Logger, ctx context.EvalInterface, conditions kyvernov1.AnyAllConditions) (bool, string) {
	anyConditions, allConditions := conditions.AnyConditions, conditions.AllConditions
	anyConditionsResult, allConditionsResult := true, true
	var messages []string

	// update the anyConditionsResult if they are present
	if anyConditions != nil {
		anyConditionsResult = false
		for _, condition := range anyConditions {
			if val, msg := Evaluate(log, ctx, condition); val {
				anyConditionsResult = true
				break
			} else {
				if msg != "" {
					messages = append(messages, msg)
				}
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
			messages = append(messages, msg)
			log.V(3).Info("a condition failed in 'all' block", "condition", condition, "message", msg)
			break
		}
	}

	finalResult := anyConditionsResult && allConditionsResult
	message := strings.Join(messages, "; ")
	return finalResult, message
}

// evaluateOldConditions evaluates multiple conditions when those conditions are provided in the old manner i.e. without 'any' or 'all'
func evaluateOldConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyvernov1.Condition) (bool, string) {
	for _, condition := range conditions {
		if val, msg := Evaluate(log, ctx, condition); !val {
			return false, msg
		}
	}

	return true, ""
}
