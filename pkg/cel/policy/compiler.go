package policy

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/autogen"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/libs/user"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

const (
	GlobalContextKey   = "globalContext"
	HttpKey            = "http"
	ImageDataKey       = "image"
	NamespaceObjectKey = "namespaceObject"
	ObjectKey          = "object"
	OldObjectKey       = "oldObject"
	RequestKey         = "request"
	ResourceKey        = "resource"
	VariablesKey       = "variables"
)

func (c *compiler) CompileValidating(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (CompiledPolicy, field.ErrorList) {
	switch policy.GetSpec().EvaluationMode() {
	case policiesv1alpha1.EvaluationModeJSON:
		return c.compileForJSON(policy, exceptions)
	default:
		return c.compileForKubernetes(policy, exceptions)
	}
}

func (c *compiler) compileForJSON(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := engine.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	variablesProvider := NewVariablesProvider(base.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider()
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	options := []cel.EnvOption{
		cel.Variable(ObjectKey, cel.DynType),
	}

	options = append(options, declOptions...)
	options = append(options, resource.Lib(), http.Lib())
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(policy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := CompileMatchConditions(path, policy.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	variables := map[string]cel.Program{}
	{
		path := path.Child("variables")
		errs := CompileVariables(path, policy.Spec.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}
	validations := make([]CompiledValidation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := CompileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}

	return &compiledPolicy{
		mode:            policiesv1alpha1.EvaluationModeJSON,
		failurePolicy:   policy.GetFailurePolicy(),
		matchConditions: matchConditions,
		variables:       variables,
		validations:     validations,
	}, nil
}

func (c *compiler) compileForKubernetes(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (CompiledPolicy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := engine.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, NamespaceType, RequestType)
	options := []cel.EnvOption{
		cel.Variable(GlobalContextKey, globalcontext.ContextType),
		cel.Variable(HttpKey, http.HTTPType),
		cel.Variable(ImageDataKey, imagedata.ContextType),
		cel.Variable(NamespaceObjectKey, NamespaceType.CelType()),
		cel.Variable(ObjectKey, cel.DynType),
		cel.Variable(OldObjectKey, cel.DynType),
		cel.Variable(RequestKey, RequestType.CelType()),
		cel.Variable(ResourceKey, resource.ContextType),
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
	options = append(options, globalcontext.Lib(), http.Lib(), imagedata.Lib(), resource.Lib(), user.Lib())
	// TODO: params, authorizer, authorizer.requestResource ?
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(policy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := CompileMatchConditions(path, policy.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	variables := map[string]cel.Program{}
	{
		path := path.Child("variables")
		errs := CompileVariables(path, policy.Spec.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}
	validations := make([]CompiledValidation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := CompileValidation(path, rule, env)
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
	autogenRules, err := vpolautogen.ValidatingPolicy(policy)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	compiledRules := map[string]compiledAutogenRule{}
	for key, rule := range autogenRules {
		rule := rule.Spec
		// compile match conditions
		matchConditions, errs := CompileMatchConditions(autogenPath.Key(key).Child("matchConditions"), rule.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		// compile variables
		variables := map[string]cel.Program{}
		errs = CompileVariables(autogenPath.Key(key).Child("variables"), rule.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		// compile validations
		validations := make([]CompiledValidation, 0, len(rule.Validations))
		for j, rule := range rule.Validations {
			path := autogenPath.Index(j).Child("validations")
			program, errs := CompileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
		// compile audit annotations
		auditAnnotations := map[string]cel.Program{}
		errs = compileAuditAnnotations(autogenPath.Key(key).Child("auditAnnotations"), rule.AuditAnnotations, env, auditAnnotations)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledRules[key] = compiledAutogenRule{
			matchConditions: matchConditions,
			variables:       variables,
			validations:     validations,
			auditAnnotation: auditAnnotations,
		}
	}

	// exceptions' match conditions
	compiledExceptions := make([]CompiledException, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), polex.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledExceptions = append(compiledExceptions, CompiledException{
			Exception:       polex,
			MatchConditions: polexMatchConditions,
		})
	}

	return &compiledPolicy{
		mode:             policiesv1alpha1.EvaluationModeKubernetes,
		failurePolicy:    policy.GetFailurePolicy(),
		matchConditions:  matchConditions,
		variables:        variables,
		validations:      validations,
		auditAnnotations: auditAnnotations,
		autogenRules:     compiledRules,
		exceptions:       compiledExceptions,
	}, nil
}

func CompileMatchConditions(path *field.Path, matchConditions []admissionregistrationv1.MatchCondition, env *cel.Env) ([]cel.Program, field.ErrorList) {
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

func CompileVariables(path *field.Path, variables []admissionregistrationv1.Variable, variablesProvider *variablesProvider, env *cel.Env, result map[string]cel.Program) field.ErrorList {
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

func CompileValidation(path *field.Path, rule admissionregistrationv1.Validation, env *cel.Env) (CompiledValidation, field.ErrorList) {
	var allErrs field.ErrorList
	compiled := CompiledValidation{
		Message: rule.Message,
	}
	{
		path = path.Child("expression")
		ast, issues := env.Compile(rule.Expression)
		if err := issues.Err(); err != nil {
			return CompiledValidation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.BoolType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.BoolType.TypeName())
			return CompiledValidation{}, append(allErrs, field.Invalid(path, rule.Expression, msg))
		}
		program, err := env.Program(ast)
		if err != nil {
			return CompiledValidation{}, append(allErrs, field.Invalid(path, rule.Expression, err.Error()))
		}
		compiled.Program = program
	}
	if rule.MessageExpression != "" {
		path = path.Child("messageExpression")
		ast, issues := env.Compile(rule.MessageExpression)
		if err := issues.Err(); err != nil {
			return CompiledValidation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, err.Error()))
		}
		if !ast.OutputType().IsExactType(types.StringType) {
			msg := fmt.Sprintf("output is expected to be of type %s", types.StringType.TypeName())
			return CompiledValidation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, msg))
		}
		program, err := env.Program(ast)
		if err != nil {
			return CompiledValidation{}, append(allErrs, field.Invalid(path, rule.MessageExpression, err.Error()))
		}
		compiled.MessageExpression = program
	}
	return compiled, nil
}
