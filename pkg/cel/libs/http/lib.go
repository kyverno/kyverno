package http

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.http"

type lib struct {
	httpIface ContextInterface
	version   *version.Version
}

func Latest() *version.Version {
	return versions.HttpVersion
}

func Lib(httpCtx ContextInterface, v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{httpIface: httpCtx, version: v})
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("http", ContextType),
		ext.NativeTypes(reflect.TypeFor[Context]()),
		c.extendEnv,
	}
}

func (l *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Globals(
			map[string]any{
				"http": l.httpIface,
			},
		),
	}
}

func (c *lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	impl := impl{
		Adapter: env.CELTypeAdapter(),
	}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"Get": {
			cel.MemberOverload(
				"http_get_string",
				[]*cel.Type{ContextType, types.StringType},
				types.AnyType,
				cel.BinaryBinding(impl.get_request_string),
			),
			cel.MemberOverload(
				"http_get_string_headers",
				[]*cel.Type{ContextType, types.StringType, types.NewMapType(types.StringType, types.StringType)},
				types.AnyType,
				cel.FunctionBinding(impl.get_request_with_headers_string),
			),
		},
		"Post": {
			cel.MemberOverload(
				"http_post_string_any",
				[]*cel.Type{ContextType, types.StringType, types.AnyType},
				types.AnyType,
				cel.FunctionBinding(impl.post_request_string),
			),
			cel.MemberOverload(
				"http_post_string_any_headers",
				[]*cel.Type{ContextType, types.StringType, types.AnyType, types.NewMapType(types.StringType, types.StringType)},
				types.AnyType,
				cel.FunctionBinding(impl.post_request_with_headers_string),
			),
		},
		"Client": {
			cel.MemberOverload(
				"http_client_string",
				[]*cel.Type{ContextType, types.StringType},
				ContextType,
				cel.BinaryBinding(impl.http_client_string),
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
