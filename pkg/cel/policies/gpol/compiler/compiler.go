package compiler

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	cellibs "github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/generator"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var (
	gpolCompilerVersion = version.MajorMinor(1, 0)
	compileError        = "generating policy compiler " + gpolCompilerVersion.String() + " error: %s"
)

type Compiler interface {
	Compile(policy *policiesv1alpha1.GeneratingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func createBaseGpolEnv(namespace string) (*environment.EnvSet, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.GeneratorKey, generator.ContextType),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
		cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
	)

	base := environment.MustBaseEnvSet(gpolCompilerVersion, false)
	env, err := base.Env(environment.StoredExpressions)
	if err != nil {
		return nil, nil, err
	}

	variablesProvider := compiler.NewVariablesProvider(env.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(compiler.NamespaceType, compiler.RequestType)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, nil, err
	}

	baseOpts = append(baseOpts, declOptions...)

	// the custom types have to be registered after the decl options have been registered, because these are what allow
	// go struct type resolution
	extendedBase, err := base.Extend(
		environment.VersionedOptions{
			IntroducedVersion: gpolCompilerVersion,
			EnvOptions:        baseOpts,
		},
		// libaries
		environment.VersionedOptions{
			IntroducedVersion: gpolCompilerVersion,
			EnvOptions: []cel.EnvOption{
				ext.NativeTypes(reflect.TypeFor[cellibs.Exception](), ext.ParseStructTags(true)),
				cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
				generator.Lib(
					generator.Latest(),
				),
				globalcontext.Lib(
					globalcontext.Latest(),
				),
				http.Lib(
					http.Latest(),
				),
				resource.Lib(
					namespace,
					resource.Latest(),
				),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Latest(),
				),
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return extendedBase, variablesProvider, nil
}

func (c *compilerImpl) Compile(policy *policiesv1alpha1.GeneratingPolicy, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	gpolEnvSet, variablesProvider, err := createBaseGpolEnv(policy.GetNamespace())
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	env, err := gpolEnvSet.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	path := field.NewPath("spec")
	// append a place holder error to the errors list to be displayed in case the error list was returned
	allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, "failed to compile policy")))

	matchConditions := make([]cel.Program, 0, len(policy.Spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := compiler.CompileMatchConditions(path, env, policy.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
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
