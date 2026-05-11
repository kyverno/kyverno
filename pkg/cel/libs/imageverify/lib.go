package imageverify

import (
	"sync/atomic"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/cel/libs/versions"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const libraryName = "kyverno.imageverify"

// ImageContextHolder holds a request-scoped ImageContext atomically
// so the compiled CEL environment can be reused across requests.
type ImageContextHolder struct {
	val atomic.Pointer[imagedataloader.ImageContext]
}

func (h *ImageContextHolder) Store(ictx imagedataloader.ImageContext) {
	h.val.Store(&ictx)
}

func (h *ImageContextHolder) Load() imagedataloader.ImageContext {
	p := h.val.Load()
	if p == nil {
		return nil
	}
	return *p
}

type lib struct {
	logger       logr.Logger
	version      *version.Version
	imgCtxHolder *ImageContextHolder
	ivpol        policiesv1beta1.ImageValidatingPolicyLike
	lister       k8scorev1.SecretInterface
}

func Latest() *version.Version {
	return versions.KyvernoLatest
}

func Lib(v *version.Version, imgCtx imagedataloader.ImageContext, ivpol policiesv1beta1.ImageValidatingPolicyLike, lister k8scorev1.SecretInterface) cel.EnvOption {
	h := &ImageContextHolder{}
	h.Store(imgCtx)
	return cel.Lib(&lib{
		version:      v,
		imgCtxHolder: h,
		ivpol:        ivpol,
		lister:       lister,
	})
}

// LibWithHolder accepts a pre-created holder so imgCtx can be swapped
// per request without recompiling the CEL environment.
func LibWithHolder(v *version.Version, holder *ImageContextHolder, ivpol policiesv1beta1.ImageValidatingPolicyLike, lister k8scorev1.SecretInterface) cel.EnvOption {
	return cel.Lib(&lib{
		version:      v,
		imgCtxHolder: holder,
		ivpol:        ivpol,
		lister:       lister,
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
	impl, err := ImageVerifyCELFuncs(c.logger, c.imgCtxHolder, c.ivpol, c.lister, env.CELTypeAdapter())
	if err != nil {
		return nil, err
	}
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
	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	return env.Extend(options...)
}
