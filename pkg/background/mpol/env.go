package mpol

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/hash"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/image"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/json"
	"github.com/kyverno/kyverno/pkg/cel/libs/math"
	"github.com/kyverno/kyverno/pkg/cel/libs/random"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/libs/time"
	"github.com/kyverno/kyverno/pkg/cel/libs/transform"
	"github.com/kyverno/kyverno/pkg/cel/libs/user"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

var targetConstraintsEnvironmentVersion = version.MajorMinor(1, 0)

func buildMpolTargetEvalEnv(namespace string) (*cel.Env, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.GlobalContextKey, globalcontext.ContextType),
		cel.Variable(compiler.HttpKey, http.ContextType),
		cel.Variable(compiler.ImageDataKey, imagedata.ContextType),
		cel.Variable(compiler.ResourceKey, resource.ContextType),
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
				random.Lib(
					random.Latest(),
				),
				time.Lib(
					time.Latest(),
				),
				transform.Lib(
					transform.Latest(),
				),
			},
		},
	)
	if err != nil {
		return nil, err
	}
	return extendedEnvSet.StoredExpressionsEnv(), nil
}
