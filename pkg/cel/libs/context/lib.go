package context

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"k8s.io/client-go/kubernetes"
)

// lib types
var ConfigMapReferenceType = types.NewObjectType("context.ConfigMapReference")

type lib struct {
	client kubernetes.Interface
}

func Lib(client kubernetes.Interface) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{
		client: client,
	})
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		ext.NativeTypes(
			// TODO: needs cel lib bump
			// ext.ParseStructTags(true),
			reflect.TypeFor[ConfigMapReference](),
		),
		// extend environment with function overloads
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
		client:  c.client,
	}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"context.configMap": {
			// TODO: add more overloads...
			cel.Overload("context_get_cm", []*cel.Type{ConfigMapReferenceType}, types.DynType, cel.UnaryBinding(impl.context_get_cm)),
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
