package compiler

import (
	"github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/libs/user"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

type Compiler interface {
	Compile(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	switch policy.GetSpec().EvaluationMode() {
	case policiesv1alpha1.EvaluationModeJSON:
		return c.compileForJSON(policy, exceptions)
	default:
		return c.compileForKubernetes(policy, exceptions)
	}
}

func (c *compilerImpl) compileForJSON(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := compiler.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	variablesProvider := compiler.NewVariablesProvider(base.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider()
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	options := []cel.EnvOption{
		cel.Variable(compiler.ObjectKey, cel.DynType),
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
		programs, errs := compiler.CompileMatchConditions(path, policy.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	variables := map[string]cel.Program{}
	{
		path := path.Child("variables")
		errs := compiler.CompileVariables(path, policy.Spec.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}
	validations := make([]compiler.Validation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}

	return &Policy{
		mode:            policiesv1alpha1.EvaluationModeJSON,
		failurePolicy:   policy.GetFailurePolicy(),
		matchConditions: matchConditions,
		variables:       variables,
		validations:     validations,
	}, nil
}

func (c *compilerImpl) compileForKubernetes(policy *policiesv1alpha1.ValidatingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	base, err := compiler.NewEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, compiler.NamespaceType, compiler.RequestType)
	options := []cel.EnvOption{
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	}
	for _, declType := range declTypes {
		options = append(options, cel.Types(declType.CelType()))
	}
	variablesProvider := compiler.NewVariablesProvider(base.CELTypeProvider())
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
		programs, errs := compiler.CompileMatchConditions(path, policy.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	variables := map[string]cel.Program{}
	{
		path := path.Child("variables")
		errs := compiler.CompileVariables(path, policy.Spec.Variables, variablesProvider, env, variables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}
	validations := make([]compiler.Validation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidation(path, rule, env)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}
	auditAnnotations := map[string]cel.Program{}
	{
		path := path.Child("auditAnnotations")
		errs := compiler.CompileAuditAnnotations(path, policy.Spec.AuditAnnotations, env, auditAnnotations)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
	}
	// // compile autogen rules
	// autogenPath := field.NewPath("status").Child("autogen").Child("rules")
	// autogenRules, err := vpolautogen.Autogen(policy)
	// if err != nil {
	// 	return nil, append(allErrs, field.InternalError(nil, err))
	// }
	// compiledRules := map[string]compiledAutogenRule{}
	// for key, rule := range autogenRules {
	// 	rule := rule.Spec
	// 	// compile match conditions
	// 	matchConditions, errs := compiler.CompileMatchConditions(autogenPath.Key(key).Child("matchConditions"), rule.MatchConditions, env)
	// 	if errs != nil {
	// 		return nil, append(allErrs, errs...)
	// 	}
	// 	// compile variables
	// 	variables := map[string]cel.Program{}
	// 	errs = compiler.CompileVariables(autogenPath.Key(key).Child("variables"), rule.Variables, variablesProvider, env, variables)
	// 	if errs != nil {
	// 		return nil, append(allErrs, errs...)
	// 	}
	// 	// compile validations
	// 	validations := make([]compiler.Validation, 0, len(rule.Validations))
	// 	for j, rule := range rule.Validations {
	// 		path := autogenPath.Index(j).Child("validations")
	// 		program, errs := compiler.CompileValidation(path, rule, env)
	// 		if errs != nil {
	// 			return nil, append(allErrs, errs...)
	// 		}
	// 		validations = append(validations, program)
	// 	}
	// 	// compile audit annotations
	// 	auditAnnotations := map[string]cel.Program{}
	// 	errs = compiler.CompileAuditAnnotations(autogenPath.Key(key).Child("auditAnnotations"), rule.AuditAnnotations, env, auditAnnotations)
	// 	if errs != nil {
	// 		return nil, append(allErrs, errs...)
	// 	}
	// 	compiledRules[key] = compiledAutogenRule{
	// 		matchConditions: matchConditions,
	// 		variables:       variables,
	// 		validations:     validations,
	// 		auditAnnotation: auditAnnotations,
	// 	}
	// }
	// exceptions' match conditions
	compiledExceptions := make([]compiler.Exception, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := compiler.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), polex.Spec.MatchConditions, env)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledExceptions = append(compiledExceptions, compiler.Exception{
			Exception:       polex,
			MatchConditions: polexMatchConditions,
		})
	}
	return &Policy{
		mode:             policiesv1alpha1.EvaluationModeKubernetes,
		failurePolicy:    policy.GetFailurePolicy(),
		matchConditions:  matchConditions,
		variables:        variables,
		validations:      validations,
		auditAnnotations: auditAnnotations,
		exceptions:       compiledExceptions,
	}, nil
}
