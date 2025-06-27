package user

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

const libraryName = "kyverno.user"

type lib struct{}

func Lib() cel.EnvOption {
	return cel.Lib(&lib{})
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[ServiceAccount]()),
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
		"parseServiceAccount": {
			cel.Overload(
				"parse_service_account_string",
				[]*cel.Type{types.StringType},
				ServiceAccountType,
				cel.UnaryBinding(impl.parse_service_account_string),
			),
		},
	}
	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	return env.Extend(options...)
}
