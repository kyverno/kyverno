package json

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

type lib struct {
	json    Json
	version *version.Version
}

func Latest() *version.Version {
	return versions.JsonVersion
}

func Lib(json JsonIface, v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{
		json:    Json{json},
		version: v,
	})
}

func (*lib) LibraryName() string {
	return "kyverno.json"
}

func (l *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("json", JsonType),
		// register native types
		ext.NativeTypes(reflect.TypeFor[Json]()),
		// extend environment with function overloads
		l.extendEnv,
	}
}

func (l *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Globals(
			map[string]any{
				"json": l.json,
			},
		),
	}
}

func (*lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	// get env type adapter
	adapter := env.CELTypeAdapter()
	// create implementation with adapter
	impl := impl{adapter}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"unmarshal": {
			cel.MemberOverload("unmarshal_string_dyn", []*cel.Type{JsonType, types.DynType}, types.DynType, cel.BinaryBinding(impl.unmarshal)),
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
