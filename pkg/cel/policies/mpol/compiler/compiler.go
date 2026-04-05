package compiler

import (
	"context"
	"fmt"
	"reflect"

	cel "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
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
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/common"
	environment "k8s.io/apiserver/pkg/cel/environment"
	"k8s.io/apiserver/pkg/cel/library"
	"k8s.io/apiserver/pkg/cel/mutation"
)

var (
	mpolCompilerVersion  = version.MajorMinor(2, 0)
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

	var patchers []Patcher
	for i, m := range policy.GetSpec().Mutations {
		switch m.PatchType {
		case admissionregistrationv1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				prog, errs := compiler.CompileMutation(path.Child("mutations").Index(i).Child("jsonPatch"), extendedCompiler, m.JSONPatch.Expression, cel.ListType(jsonPatchType))
				if errs != nil {
					return nil, append(allErrs, errs...)
				}
				patchers = append(patchers, newJSONPatcher(prog))
			}
		case admissionregistrationv1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				prog, errs := compiler.CompileMutation(path.Child("mutations").Index(i).Child("applyConfiguration"), extendedCompiler, m.ApplyConfiguration.Expression, applyConfigObjectType)
				if errs != nil {
					return nil, append(allErrs, errs...)
				}
				patchers = append(patchers, newApplyConfigPatcher(prog))
			}
		}
	}

	return &Policy{
		matchConditions:  matchConditions,
		variables:        variables,
		exceptions:       compiledExceptions,
		matchConstraints: policy.GetSpec().MatchConstraints,
		patchers:         patchers,
	}, allErrs
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
		cel.Types(applyConfigObjectType),
		cel.Types(jsonPatchType),
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
	baseOpts = append(baseOpts, common.ResolverEnvOption(&mutation.DynamicTypeResolver{}))

	// http.Get/Post are gated by scope and operator configuration (CVE-2026-4789).
	// Namespaced policies cannot use http.* unless explicitly enabled via --allowHTTPInNamespacedPolicies.
	libEnvOpts := []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
		cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
		environment.UnversionedLib(library.JSONPatch), // the kubernetes jsonpatch library to enable escapeKey
		generator.Lib(
			generator.Context{ContextInterface: libCtx},
			generator.Latest(),
		),
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: libCtx},
			globalcontext.Latest(),
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
		user.Lib(
			user.Latest(),
		),
	}
	if namespace == "" || toggle.FromContext(context.TODO()).AllowHTTPInNamespacedPolicies() {
		httpCtx, err := compiler.NewCELHTTPContext()
		if err != nil {
			return nil, nil, err
		}
		libEnvOpts = append(libEnvOpts, http.Lib(
			http.Context{ContextInterface: httpCtx},
			http.Latest(),
		))
	}

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
			EnvOptions: libEnvOpts,
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
