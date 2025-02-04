package exception

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel"
	policy "github.com/kyverno/kyverno/pkg/cel/policy"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type Compiler interface {
	Compile(*kyvernov2alpha1.CELPolicyException) (*CompiledException, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compiler{}
}

type compiler struct{}

func (c *compiler) Compile(exception *kyvernov2alpha1.CELPolicyException) (*CompiledException, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := engine.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	options := []cel.EnvOption{
		cel.Variable(policy.ObjectKey, cel.DynType),
	}
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec.matchConditions")
	matchConditions := make([]cel.Program, 0, len(exception.Spec.MatchConditions))
	for i, matchCondition := range exception.Spec.MatchConditions {
		path := path.Index(i).Child("expression")
		ast, issues := env.Compile(matchCondition.Expression)
		if err := issues.Err(); err != nil {
			return nil, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return nil, append(allErrs, field.Invalid(path, matchCondition.Expression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, matchCondition.Expression, err.Error()))
		}
		matchConditions = append(matchConditions, prog)
	}
	return &CompiledException{
		matchConditions: matchConditions,
	}, nil
}
