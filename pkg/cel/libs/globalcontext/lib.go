package globalcontext

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

const libraryName = "kyverno.globalcontext"

type lib struct{}

func Lib() cel.EnvOption {
	return cel.Lib(&lib{})
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[Context]()),
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
		"Get": {
			cel.MemberOverload(
				"globalcontext_get_string",
				[]*cel.Type{ContextType, types.StringType},
				types.DynType,
				cel.BinaryBinding(impl.get_string),
			),
			cel.MemberOverload(
				"globalcontext_get_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType},
				types.DynType,
				cel.FunctionBinding(impl.get_string_string),
			),
		},
	}
	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	return env.Extend(options...)
}
