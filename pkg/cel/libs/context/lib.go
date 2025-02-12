package context

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

const libraryName = "kyverno.context"

type lib struct{}

func Lib() cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{})
}

func Types() []*apiservercel.DeclType {
	return []*apiservercel.DeclType{
		configMapType,
		imageDataType,
	}
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
		"GetConfigMap": {
			cel.MemberOverload("get_configmap_string_string", []*cel.Type{ContextType, types.StringType, types.StringType}, configMapType.CelType(), cel.FunctionBinding(impl.get_configmap_string_string)),
		},
		"GetGlobalReference": {
			// TODO: should not use DynType in return
			cel.MemberOverload("get_globalreference_string", []*cel.Type{ContextType, types.StringType, types.StringType}, types.DynType, cel.FunctionBinding(impl.get_globalreference_string)),
		},
		"GetImageData": {
			// TODO: should not use DynType in return
			cel.MemberOverload("get_imagedata_string", []*cel.Type{ContextType, types.StringType}, imageDataType.CelType(), cel.BinaryBinding(impl.get_imagedata_string)),
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
