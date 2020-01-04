package variables

import (
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
