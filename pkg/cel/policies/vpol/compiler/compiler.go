package compiler

import (
	"github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
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
	base, err := compiler.NewBaseEnv()
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
	options = append(options, http.Lib(), image.Lib(), resource.Lib())
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(policy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := compiler.CompileMatchConditions(path, env, policy.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}

	env, err = env.Extend(
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, policy.Spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	validations := make([]compiler.Validation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidation(path, env, rule)
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
	base, err := compiler.NewBaseEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	options := []cel.EnvOption{
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
	}
	var declTypes []*apiservercel.DeclType
	declTypes = append(declTypes, compiler.NamespaceType, compiler.RequestType)
	for _, declType := range declTypes {
		options = append(options, cel.Types(declType.CelType()))
	}
	variablesProvider := compiler.NewVariablesProvider(base.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(declTypes...)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	options = append(options, declOptions...)
	// TODO: params, authorizer, authorizer.requestResource ?
	env, err := base.Extend(options...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	path := field.NewPath("spec")
	matchConditions := make([]cel.Program, 0, len(policy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := compiler.CompileMatchConditions(path, env, policy.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}
	env, err = env.Extend(
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
		globalcontext.Lib(),
		http.Lib(),
		image.Lib(),
		imagedata.Lib(),
		resource.Lib(),
		user.Lib(),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, policy.Spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	validations := make([]compiler.Validation, 0, len(policy.Spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range policy.Spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidation(path, env, rule)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}
	auditAnnotations, errs := compiler.CompileAuditAnnotations(path.Child("auditAnnotations"), env, policy.Spec.AuditAnnotations...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	// exceptions' match conditions
	compiledExceptions := make([]compiler.Exception, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := compiler.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), env, polex.Spec.MatchConditions...)
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
