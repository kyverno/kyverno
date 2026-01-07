package time

import (
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/libs/versions"
	"github.com/kyverno/kyverno/pkg/cel/utils"
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
				cel.FunctionBinding(func(...ref.Val) ref.Val {
					return e.CELTypeAdapter().NativeToValue(types.Timestamp{Time: time.Now()})
				})))

			truncateFunc := cel.Function("truncate", cel.Overload(
				"time_truncate",
				[]*cel.Type{types.TimestampType},
				types.TimestampType,
				cel.BinaryBinding(func(arg1 ref.Val, arg2 ref.Val) ref.Val {
					ts, err := utils.ConvertToNative[time.Time](arg1)
					if err != nil {
						return types.WrapErr(err)
					}
					dur, err := utils.ConvertToNative[time.Duration](arg2)
					return e.CELTypeAdapter().NativeToValue(types.Timestamp{Time: ts.Truncate(dur)})
				})))
			return e.Extend(nowFunc, truncateFunc)
		},
	}
}

func (*lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
