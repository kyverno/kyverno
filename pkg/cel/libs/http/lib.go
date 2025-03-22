package http

import (
	"net/http"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

const libraryName = "kyverno.http"

type lib struct{}

func Lib() cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{})
}

func Types() []*apiservercel.DeclType {
	return []*apiservercel.DeclType{}
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		ext.NativeTypes(reflect.TypeFor[http.Request]()),
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
		"Get": {
			cel.MemberOverload("get_request_string", []*cel.Type{HTTPType, types.StringType}, types.NewMapType(types.StringType, types.AnyType), cel.BinaryBinding(impl.get_request_string)),
			cel.MemberOverload("get_request_with_headers_string", []*cel.Type{HTTPType, types.StringType, types.NewMapType(types.StringType, types.StringType)}, types.NewMapType(types.StringType, types.AnyType), cel.FunctionBinding(impl.get_request_with_headers_string)),
		},
		"Post": {
			cel.MemberOverload("post_request_string", []*cel.Type{HTTPType, types.StringType, types.NewMapType(types.StringType, types.AnyType)}, types.NewMapType(types.StringType, types.AnyType), cel.FunctionBinding(impl.post_request_string)),
			cel.MemberOverload("post_request__with_headers_string", []*cel.Type{HTTPType, types.StringType, types.NewMapType(types.StringType, types.AnyType), types.NewMapType(types.StringType, types.StringType)}, types.NewMapType(types.StringType, types.AnyType), cel.FunctionBinding(impl.post_request_with_headers_string)),
		},
		"Client": {
			cel.MemberOverload("http_client_string", []*cel.Type{HTTPType, types.StringType}, HTTPType, cel.BinaryBinding(impl.http_client_string)),
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
