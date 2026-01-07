package time

import (
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"k8s.io/apimachinery/pkg/util/version"
)

const libraryName = "kyverno.time"

type lib struct {
	version *version.Version
}

func Lib(v *version.Version) cel.EnvOption {
	// create the cel lib env option
	return cel.Lib(&lib{version: v})
}

func Latest() *version.Version {
	return versions.TimeVersion
}

func (*lib) LibraryName() string {
	return libraryName
}

func (c *lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		func(e *cel.Env) (*cel.Env, error) {
			nowFunc := cel.Function("now", cel.Overload(
				"time_now",
				[]*cel.Type{},
				types.TimestampType,
				cel.FunctionBinding(func(...ref.Val) ref.Val { return e.CELTypeAdapter().NativeToValue(types.Timestamp{Time: time.Now()}) })))
			return e.Extend(nowFunc)
		},
	}
}

func (*lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
