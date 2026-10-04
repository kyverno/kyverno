package time

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
)

const libraryName = "kyverno.time"

type lib struct{}

func Lib() cel.EnvOption {
	return cel.Lib(&lib{})
}

func (*lib) LibraryName() string {
	return libraryName
}

func (*lib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("time_now",
			cel.Overload("time_now",
				[]*cel.Type{},
				cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return time_now()
				}),
			),
		),
		cel.Function("time_now_utc",
			cel.Overload("time_now_utc",
				[]*cel.Type{},
				cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return time_now_utc()
				}),
			),
		),
		cel.Function("time_parse",
			cel.Overload("time_parse_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(time_parse),
			),
		),
		cel.Function("time_add",
			cel.Overload("time_add_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(time_add),
			),
		),
		cel.Function("time_diff",
			cel.Overload("time_diff_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.StringType,
				cel.BinaryBinding(time_diff),
			),
		),
		cel.Function("time_before",
			cel.Overload("time_before_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(time_before),
			),
		),
		cel.Function("time_after",
			cel.Overload("time_after_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(time_after),
			),
		),
	}
}

func (*lib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}