package imageverify

import (
	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	apiservercel "k8s.io/apiserver/pkg/cel"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const libraryName = "kyverno.imageverify"

type lib struct {
	logger logr.Logger
	imgCtx imagedataloader.ImageContext
	ivpol  *v1alpha1.ImageValidatingPolicy
	lister k8scorev1.SecretInterface
}

func Lib(imgCtx imagedataloader.ImageContext, ivpol *v1alpha1.ImageValidatingPolicy, lister k8scorev1.SecretInterface) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{
		imgCtx: imgCtx,
		ivpol:  ivpol,
		lister: lister,
	})
}

func Types() []*apiservercel.DeclType {
	return []*apiservercel.DeclType{}
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		c.extendEnv,
	}
}

func (*lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func (c *lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	// create implementation, recording the envoy types aware adapter
	impl, err := ImageVerifyCELFuncs(c.logger, c.imgCtx, c.ivpol, c.lister, env.CELTypeAdapter())
	if err != nil {
		return nil, err
	}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"verifyImageSignatures": {
			cel.Overload(
				"verify_image_signature_string_stringarray",
				[]*cel.Type{types.StringType, types.NewListType(types.DynType)},
				types.IntType,
				cel.BinaryBinding(impl.verify_image_signature_string_stringarray),
			),
		},
		"verifyAttestationSignatures": {
			cel.Overload(
				"verify_image_attestations_string_string_stringarray",
				[]*cel.Type{types.StringType, types.StringType, types.NewListType(types.DynType)},
				types.IntType,
				cel.FunctionBinding(impl.verify_image_attestations_string_string_stringarray),
			),
		},
		"getImageData": {
			cel.Overload(
				"get_image_data_string",
				[]*cel.Type{types.StringType},
				types.DynType,
				cel.UnaryBinding(impl.get_image_data_string),
			),
		},
		"extractPayload": {
			cel.Overload(
				"payload_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.DynType,
				cel.BinaryBinding(impl.payload_string_string),
			),
		},
	}
	// create env options corresponding to our function overloads
	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	// extend environment with our function overloads
	return env.Extend(options...)
}
