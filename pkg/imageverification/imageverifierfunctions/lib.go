package imageverifierfunctions

import (
	"context"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	apiservercel "k8s.io/apiserver/pkg/cel"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const libraryName = "kyverno.imageverify"

type lib struct {
	ctx    context.Context
	imgCtx imagedataloader.ImageContext
	ivpol  *v1alpha1.ImageVerificationPolicy
	lister v1.SecretInterface
}

func Lib(ctx context.Context, imgCtx imagedataloader.ImageContext, ivpol *v1alpha1.ImageVerificationPolicy, lister v1.SecretInterface) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{
		ctx:    ctx,
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
	impl := ImageVerifyCELFuncs(c.ctx, c.imgCtx, c.ivpol, c.lister, env.CELTypeAdapter())
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"VerifyImageSignatures": {
			cel.MemberOverload("verify_image_signature_string_stringarray", []*cel.Type{types.StringType, types.ListType}, types.IntType, cel.BinaryBinding(impl.verify_image_signature_string_stringarray)),
		},
		"VerifyAttestationSignatures": {
			// TODO: should not use DynType in return
			cel.MemberOverload("verify_image_attestations_string_string_stringarray", []*cel.Type{types.StringType, types.StringType, types.ListType}, types.IntType, cel.FunctionBinding(impl.verify_image_attestations_string_string_stringarray)),
		},
		"GetImageData": {
			// TODO: should not use DynType in return
			cel.MemberOverload("get_image_data_string", []*cel.Type{types.StringType}, types.DynType, cel.BinaryBinding(impl.get_image_data_string)),
		},
		"Payload": {
			// TODO: should not use DynType in return
			cel.MemberOverload("payload_string_string", []*cel.Type{types.StringType, types.StringType}, types.DynType, cel.BinaryBinding(impl.payload_string_string)),
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
