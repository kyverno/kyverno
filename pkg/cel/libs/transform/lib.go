package transform

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.transform"

type lib struct {
	version *version.Version
}

func Lib(v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{version: v})
}

func Latest() *version.Version {
	return versions.TransformVersion
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
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

	return env.Extend(
		cel.Function(
			"listObjToMap",
			cel.Overload(
				"list_of_object_to_map",
				[]*cel.Type{types.ListType, types.ListType, types.StringType, types.StringType},
				types.MapType,
				cel.FunctionBinding(impl.list_of_objects_to_map),
			),
		),
	)
}
