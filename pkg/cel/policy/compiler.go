package policy

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/autogen"
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
	Compile(*policiesv1alpha1.ValidatingPolicy, []policiesv1alpha1.CELPolicyException) (CompiledPolicy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compiler{}
}

type compiler struct{}

func (c *compiler) Compile(policy *policiesv1alpha1.ValidatingPolicy, exceptions []policiesv1alpha1.CELPolicyException) (CompiledPolicy, field.ErrorList) {
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
		programs, errs := compileMatchConditions(path, policy.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	variables := map[string]cel.Program{}
	{
		path := path.Child("variables")
		errs := compileVariables(path, policy.Spec.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
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
		errs := compileAuditAnnotations(path, policy.Spec.AuditAnnotations, env, auditAnnotations)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}

	// compile autogen rules
	autogenPath := field.NewPath("status").Child("autogen").Child("rules")
	autogenRules := vpolautogen.ComputeRules(policy)
	compiledRules := make([]compiledAutogenRule, 0, len(autogenRules))
	for i, rule := range autogenRules {
		// compile match conditions
		matchConditions, errs := compileMatchConditions(autogenPath.Index(i).Child("matchConditions"), rule.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		// compile variables
		variables := map[string]cel.Program{}
		errs = compileVariables(autogenPath.Index(i).Child("variables"), rule.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		// compile validations
		validations := make([]compiledValidation, 0, len(rule.Validations))
		for j, rule := range rule.Validations {
			path := autogenPath.Index(j).Child("validations")
			program, errs := compileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
		// compile audit annotations
		auditAnnotations := map[string]cel.Program{}
		errs = compileAuditAnnotations(autogenPath.Index(i).Child("auditAnnotations"), rule.AuditAnnotation, env, auditAnnotations)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledRules = append(compiledRules, compiledAutogenRule{
			matchConditions: matchConditions,
			variables:       variables,
			validations:     validations,
			auditAnnotation: auditAnnotations,
		})
	}

	// exceptions' match conditions
	compiledExceptions := make([]compiledException, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := compileMatchConditions(field.NewPath("spec").Child("matchConditions"), polex.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledExceptions = append(compiledExceptions, compiledException{
			exception:       polex,
			matchConditions: polexMatchConditions,
		})
	}

	return &compiledPolicy{
		failurePolicy:    policy.GetFailurePolicy(),
		matchConditions:  matchConditions,
		variables:        variables,
		validations:      validations,
		auditAnnotations: auditAnnotations,
		autogenRules:     compiledRules,
		exceptions:       compiledExceptions,
	}, nil
}

func compileMatchConditions(path *field.Path, matchConditions []admissionregistrationv1.MatchCondition, env *cel.Env) ([]cel.Program, field.ErrorList) {
	var allErrs field.ErrorList
	result := make([]cel.Program, 0, len(matchConditions))
	for i, matchCondition := range matchConditions {
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
		result = append(result, prog)
	}
	return result, nil
}

func compileVariables(path *field.Path, variables []admissionregistrationv1.Variable, variablesProvider *variablesProvider, env *cel.Env, result map[string]cel.Program) field.ErrorList {
	var allErrs field.ErrorList
	for i, variable := range variables {
		path := path.Index(i).Child("expression")
		ast, issues := env.Compile(variable.Expression)
		if err := issues.Err(); err != nil {
			return append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		variablesProvider.RegisterField(variable.Name, ast.OutputType())
		prog, err := env.Program(ast)
		if err != nil {
			return append(allErrs, field.Invalid(path, variable.Expression, err.Error()))
		}
		result[variable.Name] = prog
	}
	return nil
}

func compileAuditAnnotations(path *field.Path, auditAnnotations []admissionregistrationv1.AuditAnnotation, env *cel.Env, result map[string]cel.Program) field.ErrorList {
	var allErrs field.ErrorList
	for i, auditAnnotation := range auditAnnotations {
		path := path.Index(i).Child("valueExpression")
		ast, issues := env.Compile(auditAnnotation.ValueExpression)
		if err := issues.Err(); err != nil {
			return append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.StringType) && !ast.OutputType().IsExactType(types.NullType) {
			msg := fmt.Sprintf("output is expected to be either of type %s or %s", types.StringType.TypeName(), types.NullType.TypeName())
			return append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, msg))
		}
		prog, err := env.Program(ast)
		if err != nil {
			return append(allErrs, field.Invalid(path, auditAnnotation.ValueExpression, err.Error()))
		}
		result[auditAnnotation.Key] = prog
	}
	return nil
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
