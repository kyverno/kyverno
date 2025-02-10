package policy

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel"
	"github.com/kyverno/kyverno/pkg/cel/libs/context"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

const (
	ContextKey         = "context"
	NamespaceObjectKey = "namespaceObject"
	ObjectKey          = "object"
	OldObjectKey       = "oldObject"
	RequestKey         = "request"
	VariablesKey       = "variables"
)

type Compiler interface {
	Compile(*kyvernov2alpha1.ValidatingPolicy, []kyvernov2alpha1.CELPolicyException) (CompiledPolicy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compiler{}
}

type compiler struct{}

func (c *compiler) Compile(policy *kyvernov2alpha1.ValidatingPolicy, exceptions []kyvernov2alpha1.CELPolicyException) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := engine.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, namespaceType, requestType)
	declTypes = append(declTypes, context.Types()...)
	options := []cel.EnvOption{
		cel.Variable(ContextKey, context.ContextType),
		cel.Variable(NamespaceObjectKey, namespaceType.CelType()),
		cel.Variable(ObjectKey, cel.DynType),
		cel.Variable(OldObjectKey, cel.DynType),
		cel.Variable(RequestKey, requestType.CelType()),
		cel.Variable(VariablesKey, VariablesType),
	}
	for _, declType := range declTypes {
		options = append(options, cel.Types(declType.CelType()))
	}
	variablesProvider := NewVariablesProvider(base.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(declTypes...)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		// TODO: proper error handling
		panic(err)
	}
	options = append(options, declOptions...)
	options = append(options, context.Lib())
	// TODO: params, authorizer, authorizer.requestResource ?
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(policy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		for i, matchCondition := range policy.Spec.MatchConditions {
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
	}
	variables := map[string]cel.Program{}
	{
		path := path.Child("variables")
		for i, variable := range policy.Spec.Variables {
			path := path.Index(i).Child("expression")
			ast, issues := env.Compile(variable.Expression)
			if err := issues.Err(); err != nil {
				return nil, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
			}
			variablesProvider.RegisterField(variable.Name, ast.OutputType())
			prog, err := env.Program(ast)
			if err != nil {
				return nil, append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
			}
			variables[variable.Name] = prog
		}
	}
	validations := make([]compiledValidation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := compileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}
	auditAnnotations := map[string]cel.Program{}
	{
		path := path.Child("auditAnnotations")
		for i, auditAnnotation := range policy.Spec.AuditAnnotations {
			path := path.Index(i).Child("valueExpression")
			ast, issues := env.Compile(auditAnnotation.ValueExpression)
			if err := issues.Err(); err != nil {
				return nil, append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
			}
			if !ast.OutputType().IsExactType(types.StringType) && !ast.OutputType().IsExactType(types.NullType) {
				msg := fmt.Sprintf("output is expected to be either of type %s or %s", types.StringType.TypeName(), types.NullType.TypeName())
				return nil, append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, msg))
			}
			prog, err := env.Program(ast)
			if err != nil {
				return nil, append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
			}
			auditAnnotations[auditAnnotation.Key] = prog
		}
	}

	// exceptions' match conditions
	var polexMatchConditions []cel.Program
	if len(exceptions) > 0 {
		for _, polex := range exceptions {
			path := field.NewPath("spec").Child("matchConditions")
			for i, matchCondition := range polex.Spec.MatchConditions {
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
				polexMatchConditions = append(polexMatchConditions, prog)
			}
		}
	}

	return &compiledPolicy{
		failurePolicy:        policy.GetFailurePolicy(),
		matchConditions:      matchConditions,
		variables:            variables,
		validations:          validations,
		auditAnnotations:     auditAnnotations,
		polexMatchConditions: polexMatchConditions,
	}, nil
}

func compileValidation(path *field.Path, rule admissionregistrationv1.Validation, env *cel.Env) (compiledValidation, field.ErrorList) {
	var allErrs field.ErrorList
	compiled := compiledValidation{
		message: rule.Message,
	}
	{
		path = path.Child("expression")
		ast, issues := env.Compile(rule.Expression)
		if err := issues.Err(); err != nil {
			return compiledValidation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return compiledValidation{}, append(allErrs, field.Invalid(path, rule.Expression, msg))
		}
		program, err := env.Program(ast)
		if err != nil {
			return compiledValidation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
		}
		compiled.program = program
	}
	if rule.MessageExpression != "" {
		path = path.Child("messageExpression")
		ast, issues := env.Compile(rule.MessageExpression)
		if err := issues.Err(); err != nil {
			return compiledValidation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.StringType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.StringType.TypeName())
			return compiledValidation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, msg))
		}
		program, err := env.Program(ast)
		if err != nil {
			return compiledValidation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, err.Error()))
		}
		compiled.messageExpression = program
	}
	return compiled, nil
}
