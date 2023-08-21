package variables

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/go-logr/logr"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

var (
	ConditionOperators = []kyverno.ConditionOperator{
		kyverno.ConditionOperator("Equal"),
		kyverno.ConditionOperator("Equals"),
		kyverno.ConditionOperator("NotEqual"),
		kyverno.ConditionOperator("NotEquals"),
		kyverno.ConditionOperator("In"),
		kyverno.ConditionOperator("AnyIn"),
		kyverno.ConditionOperator("AllIn"),
		kyverno.ConditionOperator("NotIn"),
		kyverno.ConditionOperator("AnyNotIn"),
		kyverno.ConditionOperator("AllNotIn"),
		kyverno.ConditionOperator("GreaterThanOrEquals"),
		kyverno.ConditionOperator("GreaterThan"),
		kyverno.ConditionOperator("LessThanOrEquals"),
		kyverno.ConditionOperator("LessThan"),
		kyverno.ConditionOperator("DurationGreaterThanOrEquals"),
		kyverno.ConditionOperator("DurationGreaterThan"),
		kyverno.ConditionOperator("DurationLessThanOrEquals"),
		kyverno.ConditionOperator("DurationLessThan"),
	}
)

func FuzzEvaluate(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		jsonData1, err := ff.GetBytes()
		if err != nil {
			return
		}
		operator, err := ff.GetInt()
		if err != nil {
			return
		}
		jsonData2, err := ff.GetBytes()
		if err != nil {
			return
		}
		o := ConditionOperators[operator%len(ConditionOperators)]
		cond := kyverno.Condition{
			RawKey:   kyverno.ToJSON(jsonData1),
			Operator: o,
			RawValue: kyverno.ToJSON(jsonData2),
		}
		ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
		_, _, _ = Evaluate(logr.Discard(), ctx, cond)
	})
}
