package yaml

import (
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

type lib struct {
	yaml    Yaml
	version *version.Version
}

func Latest() *version.Version {
	return versions.YamlVersion
}

func Lib(yaml YamlIface, v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{
		yaml:    Yaml{yaml},
		version: v,
	})
}

func (*lib) LibraryName() string {
	return "kyverno.yaml"
}

func (l *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Variable("yaml", YamlType),
		// register native types
		ext.NativeTypes(reflect.TypeFor[Yaml]()),
		// extend environment with function overloads
		l.extendEnv,
	}
}

func (l *lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{
		cel.Globals(
			map[string]any{
				"yaml": l.yaml,
			},
		),
	}
}

func (*lib) extendEnv(env *cel.Env) (*cel.Env, error) {
	adapter := env.CELTypeAdapter()
	impl := impl{adapter}
	// build our function overloads
	libraryDecls := map[string][]cel.FunctionOpt{
		"parse": {
			cel.MemberOverload("parse_string_dyn", []*cel.Type{YamlType, types.StringType}, types.DynType, cel.BinaryBinding(impl.parse)),
		},
	}

	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}
	// extend environment with our function overloads
	return env.Extend(options...)
}
