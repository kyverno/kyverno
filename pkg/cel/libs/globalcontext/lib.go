package globalcontext

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.globalcontext"

type lib struct {
	globalcontextIface ContextInterface
	version            *version.Version
}

func Lib(globalcontextCtx ContextInterface, v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{globalcontextIface: globalcontextCtx, version: v})
}

func Latest() *version.Version {
	return versions.GlobalContextVersion
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("globalContext", ContextType),
		ext.NativeTypes(reflect.TypeFor[Context]()),
		c.extendEnv,
	}
}

func (l *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Globals(
			map[string]any{
				"globalContext": l.globalcontextIface,
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
	// create env options corresponding to our function overloads
	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	// extend environment with our function overloads
	return env.Extend(options...)
}
