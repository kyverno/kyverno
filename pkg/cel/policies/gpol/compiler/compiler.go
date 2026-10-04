package compiler

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/template"
	"github.com/kyverno/sdk/extensions/cel/libs/generator"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/kyverno/sdk/extensions/cel/libs/gzip"
	"github.com/kyverno/sdk/extensions/cel/libs/hash"
	"github.com/kyverno/sdk/extensions/cel/libs/http"
	"github.com/kyverno/sdk/extensions/cel/libs/image"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"github.com/kyverno/sdk/extensions/cel/libs/json"
	"github.com/kyverno/sdk/extensions/cel/libs/math"
	"github.com/kyverno/sdk/extensions/cel/libs/random"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/libs/time"
	"github.com/kyverno/sdk/extensions/cel/libs/transform"
	"github.com/kyverno/sdk/extensions/cel/libs/x509"
	"github.com/kyverno/sdk/extensions/cel/libs/yaml"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

var (
	gpolCompilerVersion = version.MajorMinor(2, 0)
	compileError        = "generating policy compiler " + gpolCompilerVersion.String() + " error: %s"
)

type Compiler interface {
	Compile(policy policiesv1beta1.GeneratingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return NewCompilerWithTrace(false)
}

// NewCompilerWithTrace is NewCompiler with decision tracing optionally turned on: when trace is
// true, match conditions, variables and generate expressions are compiled with state tracking and
// keep their ASTs, so each evaluation can record what every expression resolved to. It is a
// separate constructor rather than a parameter on NewCompiler so the existing callers
// (controllers, CLI, tests) are untouched and stay untraced.
func NewCompilerWithTrace(trace bool) Compiler {
	return &compilerImpl{trace: trace}
}

type compilerImpl struct{ trace bool }

func (c *compilerImpl) createBaseGpolEnv(libsctx libs.Context, namespace string) (*cel.Env, *compiler.VariablesProvider, error) {
	baseOpts := compiler.EnvOptionsForVersion(
		gpolCompilerVersion,
		compiler.VersionedEnvOptions{
			IntroducedVersion: version.MajorMinor(1, 0),
			EnvOptions:        compiler.DynamicResourceEnvOptionsWithCompat(),
		},
	)
	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)

	baseEnv, err := cel.NewEnv(baseOpts...)
	if err != nil {
		return nil, nil, err
	}

	variablesProvider := compiler.NewVariablesProvider(baseEnv.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(compiler.NamespaceType, compiler.RequestType)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, nil, err
	}

	baseOpts = append(baseOpts, declOptions...)

	libEnvOpts := compiler.EnvOptionsForVersion(
		gpolCompilerVersion,
		compiler.VersionedEnvOptions{
			IntroducedVersion: version.MajorMinor(1, 0),
			EnvOptions: []cel.EnvOption{
				ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
				cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
				generator.Lib(
					generator.Context{ContextInterface: libsctx},
					namespace,
					generator.Latest(),
				),
				globalcontext.Lib(
					globalcontext.Context{ContextInterface: compiler.ConfineGlobalContext(libsctx, namespace)},
					globalcontext.Latest(),
				),
				resource.Lib(
					resource.Context{ContextInterface: libsctx},
					namespace,
					resource.Latest(),
				),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Context{ContextInterface: libsctx},
					imagedata.Latest(),
					nil,
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
				http.Lib(
					http.Context{ContextInterface: libs.NewMockAwareHTTPContext(compiler.NewLazyCELHTTPContext(namespace), libsctx.GetHTTPMocks())},
					http.Latest(),
				),
			},
		},
	)
	// the custom types have to be registered after the decl options have been registered, because these are what allow
	// go struct type resolution
	finalOpts := append(baseOpts, libEnvOpts...)
	extendedBase, err := cel.NewEnv(finalOpts...)
	if err != nil {
		return nil, nil, err
	}
	return extendedBase, variablesProvider, nil
}

func (c *compilerImpl) Compile(policy policiesv1beta1.GeneratingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	env, variablesProvider, err := c.createBaseGpolEnv(libs.GetLibsCtx(), policy.GetNamespace())
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	path := field.NewPath("spec")
	// append a place holder error to the errors list to be displayed in case the error list was returned
	allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, "failed to compile policy")))

	spec := policy.GetSpec()

	matchConditions := make([]cel.Program, 0, len(spec.MatchConditions))
	var tracedMatchConditions []compiler.TracedProgram
	{
		path := path.Child("matchConditions")
		programs, traced, errs := compiler.CompileMatchConditionsWithTrace(path, env, c.trace, spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
		tracedMatchConditions = traced
	}

	variables, tracedVariables, errs := compiler.CompileVariablesWithTrace(path.Child("variables"), env, variablesProvider, c.trace, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	generations := make([]Generation, 0, len(spec.Generation))
	{
		path := path.Child("generate")
		for i, generation := range spec.Generation {
			entryPath := path.Index(i)
			switch {
			case generation.Template != nil && generation.Expression != "":
				return nil, append(allErrs, field.Invalid(entryPath, generation, "only one of expression or template may be set"))
			case generation.Template != nil:
				tpl, errs := template.Compile(entryPath.Child("template"), env, generation.Template)
				if errs != nil {
					return nil, append(allErrs, errs...)
				}
				generations = append(generations, Generation{template: tpl, name: fmt.Sprintf("generate[%d] (template)", i)})
			case generation.Expression != "":
				traced, errs := compiler.CompileGenerationWithTrace(entryPath, env, generation, c.trace)
				if errs != nil {
					return nil, append(allErrs, errs...)
				}
				generations = append(generations, Generation{expression: traced.Program, traced: traced.Traced, ast: traced.AST, name: fmt.Sprintf("generate[%d] (expression)", i)})
			default:
				return nil, append(allErrs, field.Required(entryPath, "one of expression or template must be set"))
			}
		}
	}
	auditAnnotations, errs := compiler.CompileAuditAnnotations(path.Child("auditAnnotations"), env, spec.AuditAnnotations...)
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
		namespace:        policy.GetNamespace(),
		matchConditions:  matchConditions,
		variables:        variables,
		generations:      generations,
		auditAnnotations: auditAnnotations,
		exceptions:       compiledExceptions,
		matchConstraints: policy.GetSpec().MatchConstraints,

		trace:                 c.trace,
		tracedMatchConditions: tracedMatchConditions,
		tracedVariables:       tracedVariables,
	}, nil
}
