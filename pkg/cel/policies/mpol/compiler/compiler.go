package compiler

import (
	"fmt"
	"reflect"

	cel "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
  mpolpatch "github.com/kyverno/kyverno/pkg/cel/policies/mpol/patch"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/cel/libs/generator"
	"github.com/kyverno/sdk/cel/libs/globalcontext"
	"github.com/kyverno/sdk/cel/libs/gzip"
	"github.com/kyverno/sdk/cel/libs/hash"
	"github.com/kyverno/sdk/cel/libs/http"
	"github.com/kyverno/sdk/cel/libs/image"
	"github.com/kyverno/sdk/cel/libs/imagedata"
	"github.com/kyverno/sdk/cel/libs/json"
	"github.com/kyverno/sdk/cel/libs/math"
	"github.com/kyverno/sdk/cel/libs/random"
	"github.com/kyverno/sdk/cel/libs/resource"
	"github.com/kyverno/sdk/cel/libs/time"
	"github.com/kyverno/sdk/cel/libs/transform"
	"github.com/kyverno/sdk/cel/libs/user"
	"github.com/kyverno/sdk/cel/libs/x509"
	"github.com/kyverno/sdk/cel/libs/yaml"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	apiservercel "k8s.io/apiserver/pkg/cel"
	environment "k8s.io/apiserver/pkg/cel/environment"
)

var (
	mpolCompilerVersion  = version.MajorMinor(2, 0)
	compileError         = "mutating policy composite compiler " + mpolCompilerVersion.String() + " error: %s"
	compileExtendedError = "mutating policy extended compiler " + mpolCompilerVersion.String() + " error: %s"
)

type Compiler interface {
	Compile(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	libCtx := libs.GetLibsCtx()

	compositedCompiler, err := newCompositeCompiler(libCtx, policy.GetNamespace())
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	extendedCompiler, variablesProvider, err := newExtendedEnv(libCtx, policy.GetNamespace())
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileExtendedError, err)))
	}

	spec := policy.GetSpec()

	path := field.NewPath("spec")

	variables, errs := compiler.CompileVariables(path.Child("variables"), extendedCompiler, variablesProvider, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	// register available variables in the composition environment
	for i, variable := range spec.Variables {
		ast, err := extendedCompiler.Compile(variable.Expression)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path.Child("variables").Index(i).Child("expression"), variable.Expression, err.String()))
		}

		compositedCompiler.CompositionEnv.AddField(variable.Name, ast.OutputType())
	}

	matchConditions := make([]cel.Program, 0, len(spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := compiler.CompileMatchConditions(path, extendedCompiler, spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}

	// exceptions' match conditions
	compiledExceptions := make([]compiler.Exception, 0, len(exceptions))
	for _, polex := range exceptions {
		polexMatchConditions, errs := compiler.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), extendedCompiler, polex.Spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		compiledExceptions = append(compiledExceptions, compiler.Exception{
			Exception:       polex,
			MatchConditions: polexMatchConditions,
		})
	}

	var patchers []patch.Patcher
	patchOptions := plugincel.OptionalVariableDeclarations{
		HasParams:     false,
		HasAuthorizer: false,
		HasPatchTypes: true,
	}

	for i, m := range policy.GetSpec().Mutations {
		switch m.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				accessor := &patch.JSONPatchCondition{Expression: m.JSONPatch.Expression}
				compileResult := compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				for _, err := range compileResult.CompilationErrors() {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("mutations").Index(i).Child("jsonPatch"),
						m.JSONPatch.Expression,
						err.Error(),
					))
				}

				patchers = append(patchers, patch.NewJSONPatcher(compileResult))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				accessor := &patch.ApplyConfigurationCondition{Expression: m.ApplyConfiguration.Expression}
				compileResult := compositedCompiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				for _, err := range compileResult.CompilationErrors() {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("mutations").Index(i).Child("applyConfiguration"),
						m.ApplyConfiguration.Expression,
						err.Error(),
					))
				}
				patchers = append(patchers, mpolpatch.NewApplyConfigurationPatcher(compileResult))
			}
		}
	}

	return &Policy{
		evaluator:        mutating.PolicyEvaluator{Matcher: nil, Mutators: patchers, CompositionEnv: compositedCompiler.CompositionEnv},
		matchConditions:  matchConditions,
		variables:        variables,
		exceptions:       compiledExceptions,
		matchConstraints: policy.GetSpec().MatchConstraints,
	}, allErrs
}

func newCompositeCompiler(libCtx libs.Context, namespace string) (*plugincel.CompositedCompiler, error) {
	baseEnvSet := environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion())
	extendedEnvSet, err := baseEnvSet.Extend(
		environment.VersionedOptions{
			IntroducedVersion: version.MajorMinor(1, 0),
			EnvOptions: []cel.EnvOption{
				cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
				cel.Variable(compiler.ObjectKey, cel.DynType),
				cel.Variable(compiler.OldObjectKey, cel.DynType),
				cel.Variable(compiler.RequestKey, compiler.OriginRequestType.CelType()),
				cel.Variable(compiler.ImagesKey, image.ImageType),
				cel.Types(compiler.NamespaceType.CelType()),
				cel.Types(compiler.OriginRequestType.CelType()),
				globalcontext.Lib(globalcontext.Context{ContextInterface: libCtx}, globalcontext.Latest()),
				http.Lib(http.Context{ContextInterface: http.NewHTTP(nil)}, http.Latest()),
				image.Lib(image.Latest()),
				imagedata.Lib(imagedata.Context{ContextInterface: libCtx}, imagedata.Latest()),
				math.Lib(math.Latest()),
				resource.Lib(resource.Context{ContextInterface: libCtx}, namespace, resource.Latest()),
				user.Lib(user.Latest()),
				json.Lib(&json.JsonImpl{}, json.Latest()),
				yaml.Lib(&yaml.YamlImpl{}, yaml.Latest()),
				random.Lib(random.Latest()),
				x509.Lib(x509.Latest()),
				time.Lib(time.Latest()),
				transform.Lib(transform.Latest()),
				gzip.Lib(gzip.Latest()),
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf(compileError, err)
	}

	compositedCompiler, err := plugincel.NewCompositedCompiler(extendedEnvSet)
	if err != nil {
		return nil, fmt.Errorf(compileError, err)
	}

	return compositedCompiler, nil
}

func newExtendedEnv(libCtx libs.Context, namespace string) (*cel.Env, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)

	base := environment.MustBaseEnvSet(mpolCompilerVersion)
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
			IntroducedVersion: mpolCompilerVersion,
			EnvOptions:        baseOpts,
		},
		// libraries
		environment.VersionedOptions{
			IntroducedVersion: mpolCompilerVersion,
			EnvOptions: []cel.EnvOption{
				ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
				cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
				generator.Lib(
					generator.Context{ContextInterface: libCtx},
					generator.Latest(),
				),
				globalcontext.Lib(
					globalcontext.Context{ContextInterface: libCtx},
					globalcontext.Latest(),
				),
				http.Lib(
					http.Context{ContextInterface: http.NewHTTP(nil)},
					http.Latest(),
				),
				resource.Lib(
					resource.Context{ContextInterface: libCtx},
					namespace,
					resource.Latest(),
				),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Context{ContextInterface: libCtx},
					imagedata.Latest(),
				),
				hash.Lib(
					hash.Latest(),
				),
				math.Lib(
					math.Latest(),
				),
				json.Lib(
					&json.JsonImpl{},
					json.Latest(),
				),
				yaml.Lib(
					&yaml.YamlImpl{},
					yaml.Latest(),
				),
				random.Lib(
					random.Latest(),
				),
				x509.Lib(
					x509.Latest(),
				),
				time.Lib(
					time.Latest(),
				),
				transform.Lib(
					transform.Latest(),
				),
				gzip.Lib(
					gzip.Latest(),
				),
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}

	extendedEnv, err := extendedBase.Env(environment.StoredExpressions)
	if err != nil {
		return nil, nil, err
	}
	return extendedEnv, variablesProvider, nil
}
