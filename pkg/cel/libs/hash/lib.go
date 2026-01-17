package hash

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.hash"

type lib struct {
	version *version.Version
}

func Lib(v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{version: v})
}

func Latest() *version.Version {
	return versions.HashVersion
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Constant("mylib_version", types.StringType, types.String("")),
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
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"sha1": {
			cel.Overload(
				"sha1_string",
				[]*cel.Type{types.StringType},
				types.StringType,
				cel.UnaryBinding(impl.sha1_string),
			),
		},
		"sha256": {
			cel.Overload(
				"sha256_string",
				[]*cel.Type{types.StringType},
				types.StringType,
				cel.UnaryBinding(impl.sha256_string),
			),
		},
		"md5": {
			cel.Overload(
				"md5_string",
				[]*cel.Type{types.StringType},
				types.StringType,
				cel.UnaryBinding(impl.md5_string),
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
