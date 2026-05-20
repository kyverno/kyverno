package compiler

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
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
	"github.com/kyverno/sdk/cel/libs/x509"
	"github.com/kyverno/sdk/cel/libs/yaml"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/mutation"
)

var compileError = "deleting policy compiler " + compiler.KyvernoVersion.String() + " error: %s"

type Compiler interface {
	Compile(policy policiesv1beta1.DeletingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(policy policiesv1beta1.DeletingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*Policy, field.ErrorList) {
	if policy == nil {
		return nil, field.ErrorList{field.Required(field.NewPath("policy"), "policy must not be nil")}
	}
	spec := policy.GetDeletingPolicySpec()
	if spec == nil {
		return nil, field.ErrorList{field.Required(field.NewPath("spec"), "spec must not be nil")}
	}
	var allErrs field.ErrorList
	env, variablesProvider, err := c.createBaseDpolEnv(libs.GetLibsCtx(), policy.GetNamespace())
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	path := field.NewPath("spec")
	// append a place holder error to the errors list to be displayed in case the error list was returned
	allErrs = append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, "failed to compile policy")))
	variables, errs := compiler.CompileVariables(path.Child("variables"), env, variablesProvider, spec.Variables...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	conditions := make([]cel.Program, 0, len(spec.Conditions))
	{
		path := path.Child("conditions")
		programs, errs := compiler.CompileMatchConditions(path, env, spec.Conditions...)
		if errs != nil {
			return nil, append(allErrs, errs...)
		}
		conditions = append(conditions, programs...)
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

func (c *compilerImpl) createBaseDpolEnv(libsctx libs.Context, namespace string) (*cel.Env, *compiler.VariablesProvider, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.NamespaceObjectKey, compiler.NamespaceType.CelType()),
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.OldObjectKey, cel.DynType),
		cel.Variable(compiler.RequestKey, compiler.RequestType.CelType()),
		cel.Types(compiler.NamespaceType.CelType()),
		cel.Types(compiler.RequestType.CelType()),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
		cel.Variable(compiler.ExceptionsKey, types.NewObjectType("compiler.Exception")),
	)

	env, err := cel.NewEnv()
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

	libEnvOpts := []cel.EnvOption{
		globalcontext.Lib(
			globalcontext.Context{ContextInterface: libsctx},
			compiler.KyvernoVersion,
		),
		image.Lib(
			compiler.KyvernoVersion,
		),
		imagedata.Lib(
			imagedata.Context{ContextInterface: libsctx},
			compiler.KyvernoVersion,
		),
		resource.Lib(
			resource.Context{ContextInterface: libsctx},
			namespace,
			compiler.KyvernoVersion,
		),
		hash.Lib(
			compiler.KyvernoVersion,
		),
		math.Lib(
			compiler.KyvernoVersion,
		),
		json.Lib(
			&json.JsonImpl{},
			compiler.KyvernoVersion,
		),
		yaml.Lib(
			&yaml.YamlImpl{},
			compiler.KyvernoVersion,
		),
		random.Lib(
			compiler.KyvernoVersion,
		),
		x509.Lib(
			compiler.KyvernoVersion,
		),
		time.Lib(
			compiler.KyvernoVersion,
		),
		transform.Lib(
			compiler.KyvernoVersion,
		),
		gzip.Lib(
			compiler.KyvernoVersion,
		),
		http.Lib(
			http.Context{ContextInterface: libs.NewMockAwareHTTPContext(compiler.NewLazyCELHTTPContext(namespace), libsctx.GetHTTPMocks())},
			compiler.KyvernoVersion,
		),
	}

	// the custom types have to be registered after the decl options have been registered, because these are what allow
	// go struct type resolution
	extendedBase, err := env.Extend(append(baseOpts, libEnvOpts...)...)
	if err != nil {
		return nil, nil, err
	}
	return extendedBase, variablesProvider, nil
}
