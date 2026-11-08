package compiler

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/gzip"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
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
	"github.com/kyverno/sdk/extensions/cel/libs/user"
	"github.com/kyverno/sdk/extensions/cel/libs/x509"
	"github.com/kyverno/sdk/extensions/cel/libs/yaml"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var (
	vpolCompilerVersion = version.MajorMinor(1, 0)
	compileError        = "validating policy compiler " + vpolCompilerVersion.String() + " error: %s"
)

type Exception struct {
	AllowedImages []string `cel:"allowedImages"`
	AllowedValues []string `cel:"allowedValues"`
}

type Compiler interface {
	Compile(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList)
}

// NewCompiler builds a vpol compiler with tracing off, as admission, background and reports use
// it. For `kyverno apply --explain`, see NewCompilerWithTrace.
func NewCompiler() Compiler {
	return NewCompilerWithTrace(false)
}

// NewCompilerWithTrace is NewCompiler with decision tracing optionally turned on, for
// `kyverno apply --explain`: when trace is true, match conditions, variables and validations keep
// their ASTs so each evaluation can record what every expression resolved to. It is a separate
// constructor so that NewCompiler, which admission, background and reports use, can never turn
// tracing on by accident.
func NewCompilerWithTrace(trace bool) Compiler {
	return &compilerImpl{trace: trace}
}

type compilerImpl struct{ trace bool }

// Compile builds an executable ValidatingPolicy for its configured evaluation mode.
func (c *compilerImpl) Compile(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	switch policy.GetValidatingPolicySpec().EvaluationMode() {
	case policieskyvernoio.EvaluationModeJSON:
		return c.compileForJSON(policy, exceptions)
	default:
		return c.compileForKubernetes(policy, exceptions)
	}
}

// compileForKubernetes compiles a ValidatingPolicy for Kubernetes-object evaluation.
func (c *compilerImpl) compileForKubernetes(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	vpolEnvSet, variablesProvider, err := createBaseVpolEnv(libs.GetLibsCtx(), policy.GetNamespace(), true)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	env, err := vpolEnvSet.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	path := field.NewPath("spec")
	spec := policy.GetValidatingPolicySpec()
	// append a place holder error to the errors list to be displayed in case the error list was returned
	allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, "failed to compile policy")))

	matchConditions := make([]cel.Program, 0, len(spec.MatchConditions))
	var tracedMatchConditions []compiler.TracedProgram
	var matchConditionHasSecretReads []bool
	{
		path := path.Child("matchConditions")
		programs, traced, secretReads, errs := compiler.CompileMatchConditionsWithSecretVariables(path, env, c.trace, nil, spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
		tracedMatchConditions = traced
		matchConditionHasSecretReads = secretReads
	}
	variables, tracedVariables, secretVariables, errs := compiler.CompileVariablesWithSecretRead(path.Child("variables"), env, variablesProvider, c.trace, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	validations := make([]compiler.Validation, 0, len(spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidationWithSecretVariables(path, env, rule, c.trace, secretVariables)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}
	auditAnnotations := make(map[string]cel.Program, len(spec.AuditAnnotations))
	var auditAnnotationHasSecretReads []bool
	for i, annotation := range spec.AuditAnnotations {
		program, secretRead, errs := compiler.CompileAuditAnnotationWithSecretVariables(path.Child("auditAnnotations").Index(i), env, annotation, secretVariables)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		auditAnnotations[annotation.Key] = program
		auditAnnotationHasSecretReads = append(auditAnnotationHasSecretReads, secretRead)
	}
	compiledExceptions, errs := compileExceptions(policy.GetNamespace(), exceptions, c.trace)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	hasSecretRead := compiler.AnySecretRead(matchConditionHasSecretReads...)
	for _, read := range secretVariables {
		hasSecretRead = hasSecretRead || read
	}
	for _, validation := range validations {
		hasSecretRead = hasSecretRead || validation.HasSecretRead || validation.MessageExpressionHasSecretRead
	}
	for _, exception := range compiledExceptions {
		hasSecretRead = hasSecretRead || compiler.AnySecretRead(exception.MatchConditionHasSecretRead...)
		for _, validation := range exception.Validations {
			hasSecretRead = hasSecretRead || validation.HasSecretRead || validation.MessageExpressionHasSecretRead
		}
	}
	hasSecretRead = hasSecretRead || compiler.AnySecretRead(auditAnnotationHasSecretReads...)
	return &Policy{
		mode:                           policieskyvernoio.EvaluationModeKubernetes,
		failurePolicy:                  policy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()),
		matchConstraints:               spec.MatchConstraints,
		matchConditions:                matchConditions,
		variables:                      variables,
		validations:                    validations,
		auditAnnotations:               auditAnnotations,
		auditAnnotationsHaveSecretRead: compiler.AnySecretRead(auditAnnotationHasSecretReads...),
		exceptions:                     compiledExceptions,
		hasSecretRead:                  hasSecretRead,
		trace:                          c.trace,
		tracedMatchConditions:          tracedMatchConditions,
		tracedVariables:                tracedVariables,
		matchConditionHasSecretReads:   matchConditionHasSecretReads,
	}, nil
}

// compileForJSON compiles a ValidatingPolicy for JSON-object evaluation.
func (c *compilerImpl) compileForJSON(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	var allErrs field.ErrorList
	vpolEnvSet, variablesProvider, err := createBaseVpolEnv(libs.GetLibsCtx(), policy.GetNamespace(), true)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	env, err := vpolEnvSet.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	path := field.NewPath("spec")
	spec := policy.GetValidatingPolicySpec()

	env, err = env.Extend(
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, err))
	}

	matchConditions := make([]cel.Program, 0, len(spec.MatchConditions))
	var tracedMatchConditions []compiler.TracedProgram
	var matchConditionHasSecretReads []bool
	{
		path := path.Child("matchConditions")
		programs, traced, secretReads, errs := compiler.CompileMatchConditionsWithSecretVariables(path, env, c.trace, nil, spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
		tracedMatchConditions = traced
		matchConditionHasSecretReads = secretReads
	}
	variables, tracedVariables, secretVariables, errs := compiler.CompileVariablesWithSecretRead(path.Child("variables"), env, variablesProvider, c.trace, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	validations := make([]compiler.Validation, 0, len(spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidationWithSecretVariables(path, env, rule, c.trace, secretVariables)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}

	compiledExceptions, errs := compileExceptions(policy.GetNamespace(), exceptions, c.trace)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	hasSecretRead := compiler.AnySecretRead(matchConditionHasSecretReads...)
	for _, read := range secretVariables {
		hasSecretRead = hasSecretRead || read
	}
	for _, validation := range validations {
		hasSecretRead = hasSecretRead || validation.HasSecretRead || validation.MessageExpressionHasSecretRead
	}
	for _, exception := range compiledExceptions {
		hasSecretRead = hasSecretRead || compiler.AnySecretRead(exception.MatchConditionHasSecretRead...)
		for _, validation := range exception.Validations {
			hasSecretRead = hasSecretRead || validation.HasSecretRead || validation.MessageExpressionHasSecretRead
		}
	}

	return &Policy{
		mode:                         policieskyvernoio.EvaluationModeJSON,
		failurePolicy:                policy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()),
		matchConditions:              matchConditions,
		variables:                    variables,
		validations:                  validations,
		exceptions:                   compiledExceptions,
		hasSecretRead:                hasSecretRead,
		trace:                        c.trace,
		tracedMatchConditions:        tracedMatchConditions,
		tracedVariables:              tracedVariables,
		matchConditionHasSecretReads: matchConditionHasSecretReads,
	}, nil
}

// createBaseVpolEnv builds the environment ValidatingPolicy expressions compile against.
// withPolicyScope declares `variables` and `exceptions`, which hold nothing until the policy runs
// and are unknowable to the exception webhook, so exception expressions compile without them.
func createBaseVpolEnv(libsctx libs.Context, namespace string, withPolicyScope bool) (*environment.EnvSet, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DefaultEnvOptionsWithCompat()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
	)
	if withPolicyScope {
		baseOpts = append(baseOpts, cel.Variable(compiler.VariablesKey, compiler.VariablesType))
	}

	base := environment.MustBaseEnvSet(vpolCompilerVersion)
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
	libEnvOpts := []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
	}
	// must be declared here, while ext.NativeTypes' provider is the active one; appending it after
	// the libraries below leaves the libs.Exception type unresolvable.
	if withPolicyScope {
		libEnvOpts = append(libEnvOpts, cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")))
	}
	libEnvOpts = append(libEnvOpts,
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
			nil, // this policy doesn't have a way to specify extra registry credentials, only the ivpol does.
		),
		user.Lib(
			user.Latest(),
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
			gzip.Latest(), 0,
		),
		http.Lib(
			http.Context{ContextInterface: libs.NewMockAwareHTTPContext(compiler.NewLazyCELHTTPContext(namespace), libsctx.GetHTTPMocks())},
			http.Latest(),
		),
	)

	// compiler.BaseEnvSet, not base.Extend: registers CostLib last, in this one Extend
	// call (see pkg/cel/compiler/env.go).
	extendedBase, err := compiler.BaseEnvSet(vpolCompilerVersion,
		environment.VersionedOptions{
			IntroducedVersion: vpolCompilerVersion,
			EnvOptions:        baseOpts,
		},
		// libraries
		environment.VersionedOptions{
			IntroducedVersion: vpolCompilerVersion,
			EnvOptions:        libEnvOpts,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	return extendedBase, variablesProvider, nil
}

// NewExceptionEnv builds the environment a PolicyException's expressions compile against.
// Exported so the webhook can reject a malformed one up front: an exception that fails to compile
// takes down every policy it references, and writing one needs access to a single namespace.
func NewExceptionEnv(namespace string) (*cel.Env, error) {
	envSet, _, err := createBaseVpolEnv(libs.GetLibsCtx(), namespace, false)
	if err != nil {
		return nil, err
	}
	return envSet.Env(environment.StoredExpressions)
}

// compileExceptions compiles exception match conditions for a namespaced policy.
func compileExceptions(namespace string, exceptions []*policiesv1beta1.PolicyException, trace bool) ([]compiler.Exception, field.ErrorList) {
	if len(exceptions) == 0 {
		return nil, nil
	}
	env, err := NewExceptionEnv(namespace)
	if err != nil {
		return nil, field.ErrorList{field.InternalError(nil, fmt.Errorf(compileError, err))}
	}
	return compiler.CompileExceptionsWithValidations(env, exceptions, trace)
}
