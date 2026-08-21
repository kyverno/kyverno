package attestation

import (
	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/kyverno/sdk/extensions/cel/libs/versions"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"k8s.io/apimachinery/pkg/util/version"
	apiservercel "k8s.io/apiserver/pkg/cel"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const libraryName = "kyverno.attestation"

type lib struct {
	logger logr.Logger
	ver    *version.Version
	imgCtx imagedataloader.ImageContext
	lister k8scorev1.SecretInterface
}

func Latest() *version.Version {
	return versions.KyvernoLatest
}

// Lib returns a CEL environment option that registers attestation verification
// functions independent of any ImageValidatingPolicy. If imgCtx is nil, an image
// context is created internally (using lister if provided).
func Lib(v *version.Version, imgCtx imagedataloader.ImageContext, lister k8scorev1.SecretInterface) cel.EnvOption {
	return cel.Lib(&lib{
		ver:    v,
		imgCtx: imgCtx,
		lister: lister,
	})
}

func Types() []*apiservercel.DeclType {
	return []*apiservercel.DeclType{}
}

func (*lib) LibraryName() string {
	return libraryName
}

func (l *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{l.extendEnv}
}

func (*lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func (l *lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	impl, err := newAtFuncs(l.logger, l.imgCtx, l.lister, env.CELTypeAdapter())
	if err != nil {
		return nil, err
	}
	return registerFuncs(env, impl)
}
