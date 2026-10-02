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
	"github.com/kyverno/kyverno/pkg/toggle"
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

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy policiesv1beta1.ValidatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	switch policy.GetValidatingPolicySpec().EvaluationMode() {
	case policieskyvernoio.EvaluationModeJSON:
		return c.compileForJSON(policy, exceptions)
	default:
		return c.compileForKubernetes(policy, exceptions)
	}
}

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
	{
		path := path.Child("matchConditions")
		programs, errs := compiler.CompileMatchConditions(path, env, spec.MatchConditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		matchConditions = append(matchConditions, programs...)
	}

	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	validations := make([]compiler.Validation, 0, len(spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidation(path, env, rule)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}
	auditAnnotations, errs := compiler.CompileAuditAnnotations(path.Child("auditAnnotations"), env, spec.AuditAnnotations...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	compiledExceptions, errs := compileExceptions(policy.GetNamespace(), exceptions)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	return &Policy{
		mode:             policieskyvernoio.EvaluationModeKubernetes,
		failurePolicy:    policy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()),
		matchConstraints: spec.MatchConstraints,
		matchConditions:  matchConditions,
		variables:        variables,
		validations:      validations,
		auditAnnotations: auditAnnotations,
		exceptions:       compiledExceptions,
	}, nil
}

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

	matchConditions := make([]cel.Program, 0, len(spec.MatchConditions))
	{
		path := path.Child("matchConditions")
		programs, errs := compiler.CompileMatchConditions(path, env, spec.MatchConditions...)
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

	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	validations := make([]compiler.Validation, 0, len(spec.Validations))
	{
		path := path.Child("validations")
		for i, rule := range spec.Validations {
			path := path.Index(i)
			program, errs := compiler.CompileValidation(path, env, rule)
			if errs != nil {
				return nil, append(allErrs, errs...)
			}
			validations = append(validations, program)
		}
	}

	compiledExceptions, errs := compileExceptions(policy.GetNamespace(), exceptions)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}

	return &Policy{
		mode:            policieskyvernoio.EvaluationModeJSON,
		failurePolicy:   policy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore()),
		matchConditions: matchConditions,
		variables:       variables,
		validations:     validations,
		exceptions:      compiledExceptions,
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
			gzip.Latest(),
		),
		http.Lib(
			http.Context{ContextInterface: libs.NewMockAwareHTTPContext(compiler.NewLazyCELHTTPContext(namespace), libsctx.GetHTTPMocks())},
			http.Latest(),
		),
	)

	extendedBase, err := base.Extend(
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

func compileExceptions(namespace string, exceptions []*policiesv1beta1.PolicyException) ([]compiler.Exception, field.ErrorList) {
	if len(exceptions) == 0 {
		return nil, nil
	}
	env, err := NewExceptionEnv(namespace)
	if err != nil {
		return nil, field.ErrorList{field.InternalError(nil, fmt.Errorf(compileError, err))}
	}
	return compiler.CompileExceptionsWithValidations(env, exceptions)
}
