package variables

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables/operator"
)

//Evaluate evaluates the condition
func Evaluate(log logr.Logger, ctx context.EvalInterface, condition kyverno.Condition) bool {
	// get handler for the operator
	handle := operator.CreateOperatorHandler(log, ctx, condition.Operator, SubstituteVars)
	if handle == nil {
		return false
	}
	return handle.Evaluate(condition.Key, condition.Value)
}

//EvaluateConditions evaluates multiple conditions as a logical AND operation
func EvaluateConditions(log logr.Logger, ctx context.EvalInterface, conditions []kyverno.Condition) bool {
	for _, condition := range conditions {
		if !Evaluate(log, ctx, condition) {
			return false
		}
	}

	return true
}
