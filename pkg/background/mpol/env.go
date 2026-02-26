package mpol

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/cel/libs/globalcontext"
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
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var targetConstraintsEnvironmentVersion = version.MajorMinor(1, 0)

func BuildMpolTargetEvalEnv(libsctx libs.Context, namespace string) (*cel.Env, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)

	base := environment.MustBaseEnvSet(targetConstraintsEnvironmentVersion)
	env, err := base.Env(environment.StoredExpressions)
	if err != nil {
		return nil, err
	}

	variablesProvider := compiler.NewVariablesProvider(env.CELTypeProvider())
	declProvider := apiservercel.NewDeclTypeProvider(compiler.NamespaceType, compiler.RequestType)
	declOptions, err := declProvider.EnvOptions(variablesProvider)
	if err != nil {
		return nil, err
	}

	baseOpts = append(baseOpts, declOptions...)

	// the custom types have to be registered after the decl options have been registered, because these are what allow
	// go struct type resolution
	extendedEnvSet, err := base.Extend(
		environment.VersionedOptions{
			IntroducedVersion: targetConstraintsEnvironmentVersion,
			EnvOptions:        baseOpts,
		},
		// libaries
		environment.VersionedOptions{
			IntroducedVersion: targetConstraintsEnvironmentVersion,
			EnvOptions: []cel.EnvOption{
				cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
				globalcontext.Lib(
					globalcontext.Context{ContextInterface: libsctx},
					globalcontext.Latest(),
				),
				http.Lib(
					http.Context{ContextInterface: http.NewHTTP(nil)},
					http.Latest(),
				),
				image.Lib(
					image.Latest(),
				),
				imagedata.Lib(
					imagedata.Context{ContextInterface: libsctx},
					imagedata.Latest(),
				),
				resource.Lib(
					resource.Context{ContextInterface: libsctx},
					namespace,
					resource.Latest(),
				),
				user.Lib(
					user.Latest(),
				),
				math.Lib(
					math.Latest(),
				),
				hash.Lib(
					hash.Latest(),
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
				time.Lib(
					time.Latest(),
				),
				transform.Lib(
					transform.Latest(),
				),
				x509.Lib(
					x509.Latest(),
				),
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return extendedEnvSet.StoredExpressionsEnv(), nil
}
