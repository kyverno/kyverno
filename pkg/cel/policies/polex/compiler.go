package polex

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
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
	"github.com/kyverno/sdk/cel/libs/time"
	"github.com/kyverno/sdk/cel/libs/transform"
	"github.com/kyverno/sdk/cel/libs/user"
	"github.com/kyverno/sdk/cel/libs/x509"
	"github.com/kyverno/sdk/cel/libs/yaml"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var (
	polexCompilerVersion = version.MajorMinor(1, 0)
	compileError         = "validating policy compiler " + polexCompilerVersion.String() + " error: %s"
)

type Compiler interface {
	Compile(exception policiesv1beta1.PolicyException) (*Exception, field.ErrorList)
}

func NewCompiler() Compiler {
	return &compilerImpl{}
}

type compilerImpl struct{}

func (c *compilerImpl) Compile(exception policiesv1beta1.PolicyException) (*Exception, field.ErrorList) {
	var allErrs field.ErrorList
	vpolEnvSet, _, err := createBaseVpolEnv(libs.GetLibsCtx())
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	env, err := vpolEnvSet.Env(environment.StoredExpressions)
	if err != nil {
		return nil, append(allErrs, field.InternalError(nil, fmt.Errorf(compileError, err)))
	}

	// exceptions' match conditions
	polexMatchConditions, errs := compiler.CompileMatchConditions(field.NewPath("spec").Child("matchConditions"), env, exception.Spec.MatchConditions...)
	if errs != nil {
		return nil, append(allErrs, errs...)
	}
	compiled := Exception{
		Exception:       &exception,
		MatchConditions: polexMatchConditions,
	}
	return &compiled, nil
}

func createBaseVpolEnv(libsctx libs.Context) (*environment.EnvSet, *compiler.VariablesProvider, error) {
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

	base := environment.MustBaseEnvSet(polexCompilerVersion)
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
			IntroducedVersion: polexCompilerVersion,
			EnvOptions:        baseOpts,
		},
		// libaries
		environment.VersionedOptions{
			IntroducedVersion: polexCompilerVersion,
			EnvOptions: []cel.EnvOption{
				ext.NativeTypes(reflect.TypeFor[libs.Exception](), ext.ParseStructTags(true)),
				cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
				globalcontext.Lib(
					globalcontext.Context{ContextInterface: libsctx},
					globalcontext.Latest(),
				),
				http.Lib(
					http.Context{ContextInterface: http.NewHTTP(nil)},
					http.Latest(),
				),
				// resource.Lib(
				// 	resource.Context{ContextInterface: libsctx},
				// 	namespace,
				// 	resource.Latest(),
				// ),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Context{ContextInterface: libsctx},
					imagedata.Latest(),
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
			},
		},
	)
	if err != nil {
		return nil, nil, err
	}

	return extendedBase, variablesProvider, nil
}
