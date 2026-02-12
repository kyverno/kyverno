package x509

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.x509"

type lib struct {
	version *version.Version
}

func Lib(v *version.Version) cel.EnvOption {
	return cel.Lib(&lib{version: v})
}

func Latest() *version.Version {
	return versions.X509Version
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
	impl := impl{
		Adapter: env.CELTypeAdapter(),
	}
	libraryDecls := map[string][]cel.FunctionOpt{
		"x509.decode": {
			cel.Overload(
				"x509_decode_string",
				[]*cel.Type{types.StringType},
				types.DynType,
				cel.UnaryBinding(impl.decode),
			),
		},
	}
	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	return env.Extend(options...)
}
