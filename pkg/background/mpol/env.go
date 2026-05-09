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
	apiservercel "k8s.io/apiserver/pkg/cel"
)

func BuildMpolTargetEvalEnv(libsctx libs.Context, namespace string) (*cel.Env, error) {
	baseOpts := compiler.DefaultEnvOptions()
	baseOpts = append(baseOpts,
		cel.Variable(compiler.ObjectKey, cel.DynType),
		cel.Variable(compiler.VariablesKey, compiler.VariablesType),
	)

	env, err := cel.NewEnv()
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

	libEnvOpts := []cel.EnvOption{
		cel.Variable(compiler.ExceptionsKey, types.NewObjectType("libs.Exception")),
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
		user.Lib(
			compiler.KyvernoVersion,
		),
		math.Lib(
			compiler.KyvernoVersion,
		),
		hash.Lib(
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
		time.Lib(
			compiler.KyvernoVersion,
		),
		transform.Lib(
			compiler.KyvernoVersion,
		),
		x509.Lib(
			compiler.KyvernoVersion,
		),
		http.Lib(
			http.Context{ContextInterface: compiler.NewLazyCELHTTPContext(namespace)},
			compiler.KyvernoVersion,
		),
	}

	// the custom types have to be registered after the decl options have been registered, because these are what allow
	// go struct type resolution
	return env.Extend(append(baseOpts, libEnvOpts...)...)
}
