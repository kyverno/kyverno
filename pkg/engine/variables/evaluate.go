package variables

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/variables/operator"
)

func Evaluate(ctx context.EvalInterface, condition kyverno.Condition) bool {
	// get handler for the operator
	handle := operator.CreateOperatorHandler(ctx, condition.Operator, SubstituteVariables)
	if handle == nil {
		return false
	}
	return handle.Evaluate(condition.Key, condition.Value)
}

func EvaluateConditions(ctx context.EvalInterface, conditions []kyverno.Condition) bool {
	// AND the conditions
	for _, condition := range conditions {
		if !Evaluate(ctx, condition) {
			glog.V(4).Infof("condition %v failed", condition)
			return false
		}
	}
	return true
}
