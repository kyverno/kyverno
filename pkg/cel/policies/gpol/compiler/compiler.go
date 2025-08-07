package compiler

import (
	"github.com/google/cel-go/cel"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/generator"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

type Compiler interface {
	Compile(policy *policiesv1alpha1.GeneratingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.GeneratingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
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
		cel.Variable(compiler.GeneratorKey, generator.ContextType),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
		generator.Lib(),
		resource.Lib(),
		globalcontext.Lib(),
		http.Lib(),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, policy.Spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	generations := make([]cel.Program, 0, len(policy.Spec.Generation))
	{
		path := path.Child("generate")
		programs, errs := compiler.CompileGenerations(path, env, policy.Spec.Generation...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		generations = append(generations, programs...)
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
		matchConditions: matchConditions,
		variables:       variables,
		generations:     generations,
		exceptions:      compiledExceptions,
	}, nil
}
