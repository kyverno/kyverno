package jmespath

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

const libraryName = "kyverno.jmespath"

type lib struct{}

func Lib() cel.EnvOption {
	return cel.Lib(&lib{})
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
	impl := &jmesfuncs{Adapter: env.CELTypeAdapter()}
	libraryDecls := map[string][]cel.FunctionOpt{
		"jmespath": {
			cel.Overload(
				"jmespath_string_dyn",
				[]*cel.Type{types.StringType, types.DynType},
				types.DynType,
				cel.BinaryBinding(impl.match_string_dyn),
			),
		},
	}
	
	var options []cel.EnvOption
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	return env.Extend(options...)
}
