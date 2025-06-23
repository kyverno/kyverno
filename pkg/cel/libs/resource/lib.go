package resource

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

const libraryName = "kyverno.resource"

type lib struct{}

func Lib() cel.EnvOption {
	// create the cel lib env option
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
	// create implementation, recording the envoy types aware adapter
	impl := impl{
		Adapter: env.CELTypeAdapter(),
	}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"List": {
			cel.MemberOverload(
				"resource_list_string_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_string_string_string),
			),
		},
		"Get": {
			cel.MemberOverload(
				"resource_get_string_string_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.StringType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.get_resource_string_string_string_string),
			),
		},
		"Post": {
			cel.MemberOverload(
				"resource_post_string_string_string_map",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.StringType, types.NewMapType(types.StringType, types.AnyType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.post_resource_string_string_string_map),
			),
			cel.MemberOverload(
				"resource_post_string_string_map",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.NewMapType(types.StringType, types.AnyType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.post_resource_string_string_map),
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
