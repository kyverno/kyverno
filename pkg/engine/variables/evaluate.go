package variables

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
)

//Evaluate evaluates the condition
func Evaluate(log logr.Logger, ctx context.EvalInterface, condition kyverno.Condition) bool {
	// get handler for the operator
	handle := operator.CreateOperatorHandler(log, ctx, condition.Operator)
	if handle == nil {
		return false
	}
	return handle.Evaluate(condition.Key, condition.Value)
}

//EvaluateConditions evaluates all the conditions present in a slice, in a backwards compatible way
func EvaluateConditions(log logr.Logger, ctx context.EvalInterface, conditions interface{}) bool {
	switch typedConditions := conditions.(type) {
	case kyverno.AnyAllConditions:
		return evaluateAnyAllConditions(log, ctx, typedConditions)
	case []kyverno.Condition: // backwards compatibility
		return evaluateOldConditions(log, ctx, typedConditions)
	}
	return false
}

func EvaluateAnyAllConditions(log logr.Logger, ctx context.EvalInterface, conditions []*kyverno.AnyAllConditions) bool {
	for _, c := range conditions {
		if !evaluateAnyAllConditions(log, ctx, *c) {
			return false
		}
	}

	return true
}

//evaluateAnyAllConditions evaluates multiple conditions as a logical AND (all) or OR (any) operation depending on the conditions
func evaluateAnyAllConditions(log logr.Logger, ctx context.EvalInterface, conditions kyverno.AnyAllConditions) bool {
	anyConditions, allConditions := conditions.AnyConditions, conditions.AllConditions
	anyConditionsResult, allConditionsResult := true, true

	// update the anyConditionsResult if they are present
	if anyConditions != nil {
		anyConditionsResult = false
		for _, condition := range anyConditions {
			if Evaluate(log, ctx, condition) {
				anyConditionsResult = true
				break
			}
		}
	}

	// update the allConditionsResult if they are present
	for _, condition := range allConditions {
		if !Evaluate(log, ctx, condition) {
			allConditionsResult = false
			break
		}
	}

	finalResult := anyConditionsResult && allConditionsResult
	return finalResult
}

//evaluateOldConditions evaluates multiple conditions when those conditions are provided in the old manner i.e. without 'any' or 'all'
func evaluateOldConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyverno.Condition) bool {
	for _, condition := range conditions {
		if !Evaluate(log, ctx, condition) {
			return false
		}
	}

	return true
}
