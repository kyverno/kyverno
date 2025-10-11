package compiler

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
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

var dpolCompilerVersion = version.MajorMinor(1, 0)

type Compiler interface {
	Compile(policy policiesv1beta1.DeletingPolicyLike, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy policiesv1beta1.DeletingPolicyLike, exceptions []*policiesv1alpha1.PolicyException) (*Policy, field.ErrorList) {
	if policy == nil {
		return nil, field.ErrorList{field.Required(field.NewPath("policy"), "policy must not be nil")}
	}
	spec := policy.GetDeletingPolicySpec()
	if spec == nil {
		return nil, field.ErrorList{field.Required(field.NewPath("spec"), "spec must not be nil")}
	}
	var allErrs field.ErrorList
	dpolEnvSet, variablesProvider, err := c.createBaseDpolEnv()
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	env, err := dpolEnvSet.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	path := field.NewPath("spec")
	conditions := make([]cel.Program, 0, len(spec.Conditions))
	{
		path := path.Child("conditions")
		programs, errs := compiler.CompileMatchConditions(path, env, spec.Conditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		conditions = append(conditions, programs...)
	}
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, spec.Variables...)
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
		deletionPropagationPolicy: spec.DeletionPropagationPolicy,
		schedule:                  spec.Schedule,
		conditions:                conditions,
		variables:                 variables,
		exceptions:                compiledExceptions,
	}, nil
}

func (c *compilerImpl) createBaseDpolEnv() (*environment.EnvSet, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
		cel.Variable(compiler.ExceptionsKey, types.NewObjectType("compiler.Exception")),
	)

	base := environment.MustBaseEnvSet(dpolCompilerVersion, false)
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
			IntroducedVersion: dpolCompilerVersion,
			EnvOptions:        baseOpts,
		},
		// libaries
		environment.VersionedOptions{
			IntroducedVersion: dpolCompilerVersion,
			EnvOptions: []cel.EnvOption{
				globalcontext.Lib(
					globalcontext.Latest(),
				),
				http.Lib(
					http.Latest(),
				),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Latest(),
				),
				resource.Lib(
					resource.Latest(),
				),
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}
	return extendedBase, variablesProvider, nil
}
