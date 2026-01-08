package resource

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.resource"

type lib struct {
	namespace string
	version   *version.Version
}

func Latest() *version.Version {
	return versions.ResourceVersion
}

func Lib(namespace string, v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{namespace: namespace, version: v})
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
	if c.namespace != "" {
		return c.namespacedEnv(env)
	}

	return c.clusterEnv(env)
}

func (c *lib) namespacedEnv(env *cel.Env) (*cel.Env, error) {
	impl := namespacedImpl{
		namespace: c.namespace,
		Adapter:   env.CELTypeAdapter(),
	}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"List": {
			cel.MemberOverload(
				"resource_list_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_string_string),
			),
			cel.MemberOverload(
				"resource_list_string_string_map",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.NewMapType(types.StringType, types.StringType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_string_string_map),
			),
			cel.MemberOverload(
				"list_resources_gvr",
				[]*cel.Type{ContextType, GVRType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_gvr),
			),
			cel.MemberOverload(
				"list_resources_gvr_map",
				[]*cel.Type{ContextType, GVRType, types.NewMapType(types.StringType, types.StringType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_gvr_map),
			),
		},
		"Get": {
			cel.MemberOverload(
				"resource_get_string_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.get_resource_string_string_string),
			),
			cel.MemberOverload(
				"resource_get_gvr_string",
				[]*cel.Type{ContextType, GVRType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.get_resources_gvr_string),
			),
		},
		"Post": {
			cel.MemberOverload(
				"resource_post_string_string_map",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.NewMapType(types.StringType, types.AnyType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.post_resource_string_string_map),
			),
		},
		"ToGVR": {
			cel.MemberOverload(
				"convert_to_gvr_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType},
				GVRType,
				cel.FunctionBinding(impl.convert_to_gvr_string_string),
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

func (c *lib) clusterEnv(env *cel.Env) (*cel.Env, error) {
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
			cel.MemberOverload(
				"resource_list_string_string_string_map",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.StringType, types.NewMapType(types.StringType, types.StringType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_string_string_string_map),
			),
			cel.MemberOverload(
				"list_resources_gvr_string",
				[]*cel.Type{ContextType, GVRType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_gvr_string),
			),
			cel.MemberOverload(
				"list_resources_gvr_string_map",
				[]*cel.Type{ContextType, GVRType, types.StringType, types.NewMapType(types.StringType, types.StringType)},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.list_resources_gvr_string_map),
			),
		},
		"Get": {
			cel.MemberOverload(
				"resource_get_string_string_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType, types.StringType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.get_resource_string_string_string_string),
			),
			cel.MemberOverload(
				"resource_get_gvr_string_string",
				[]*cel.Type{ContextType, GVRType, types.StringType, types.StringType},
				types.NewMapType(types.StringType, types.AnyType),
				cel.FunctionBinding(impl.get_resources_gvr_string_string),
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
		"ToGVR": {
			cel.MemberOverload(
				"convert_to_gvr_string_string",
				[]*cel.Type{ContextType, types.StringType, types.StringType},
				GVRType,
				cel.FunctionBinding(impl.convert_to_gvr_string_string),
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
