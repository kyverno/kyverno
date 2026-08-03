package time

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

const libraryName = "kyverno.time"

type lib struct{}

func Lib() cel.EnvOption {
	return cel.Lib(&lib{})
}

func Types() []interface{} {
	return []interface{}{}
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
	impl, err := NewTimeLib(env.CELTypeAdapter())
	if err != nil {
		return nil, err
	}

	libraryDecls := map[string][]cel.FunctionOpt{
		"time_now": {
			cel.Overload(
				"time_now",
				[]*cel.Type{},
				types.StringType,
				cel.FunctionBinding(impl.time_now),
			),
		},
		"time_now_utc": {
			cel.Overload(
				"time_now_utc",
				[]*cel.Type{},
				types.StringType,
				cel.FunctionBinding(impl.time_now_utc),
			),
		},
		"time_parse": {
			cel.Overload(
				"time_parse_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.StringType,
				cel.BinaryBinding(impl.time_parse),
			),
		},
		"time_add": {
			cel.Overload(
				"time_add_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.StringType,
				cel.BinaryBinding(impl.time_add),
			),
		},
		"time_diff": {
			cel.Overload(
				"time_diff_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.StringType,
				cel.BinaryBinding(impl.time_diff),
			),
		},
		"time_before": {
			cel.Overload(
				"time_before_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.BoolType,
				cel.BinaryBinding(impl.time_before),
			),
		},
		"time_after": {
			cel.Overload(
				"time_after_string_string",
				[]*cel.Type{types.StringType, types.StringType},
				types.BoolType,
				cel.BinaryBinding(impl.time_after),
			),
		},
	}

	options := []cel.EnvOption{}
	for name, overloads := range libraryDecls {
		options = append(options, cel.Function(name, overloads...))
	}

	return env.Extend(options...)
}

type TimeLib struct {
	adapter types.Adapter
}

func NewTimeLib(adapter types.Adapter) (*TimeLib, error) {
	return &TimeLib{adapter: adapter}, nil
}

func (t *TimeLib) time_now(args ...ref.Val) ref.Val {
	return types.String(time.Now().Format(time.RFC3339))
}

func (t *TimeLib) time_now_utc(args ...ref.Val) ref.Val {
	return types.String(time.Now().UTC().Format(time.RFC3339))
}

func (t *TimeLib) time_parse(layout, value ref.Val) ref.Val {
	layoutStr, ok := layout.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("layout must be a string"))
	}
	valueStr, ok := value.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("value must be a string"))
	}

	_, err := strconv.ParseInt(layoutStr, 10, 64)
	if err == nil {
		epochTime, err := strconv.ParseInt(valueStr, 10, 64)
		if err != nil {
			return types.WrapErr(fmt.Errorf("invalid epoch timestamp: %w", err))
		}
		return types.String(time.Unix(epochTime, 0).UTC().Format(time.RFC3339))
	}

	parsed, err := time.Parse(layoutStr, valueStr)
	if err != nil {
		return types.WrapErr(fmt.Errorf("failed to parse time: %w", err))
	}

	return types.String(parsed.Format(time.RFC3339))
}

func (t *TimeLib) time_add(timestamp, duration ref.Val) ref.Val {
	ts, ok := timestamp.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("timestamp must be a string"))
	}
	dur, ok := duration.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("duration must be a string"))
	}

	tm, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid timestamp: %w", err))
	}

	d, err := time.ParseDuration(dur)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid duration: %w", err))
	}

	return types.String(tm.Add(d).Format(time.RFC3339))
}

func (t *TimeLib) time_diff(t1, t2 ref.Val) ref.Val {
	ts1, ok := t1.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("first timestamp must be a string"))
	}
	ts2, ok := t2.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("second timestamp must be a string"))
	}

	time1, err := time.Parse(time.RFC3339, ts1)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid first timestamp: %w", err))
	}

	time2, err := time.Parse(time.RFC3339, ts2)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid second timestamp: %w", err))
	}

	return types.String(time2.Sub(time1).String())
}

func (t *TimeLib) time_before(t1, t2 ref.Val) ref.Val {
	ts1, ok := t1.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("first timestamp must be a string"))
	}
	ts2, ok := t2.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("second timestamp must be a string"))
	}

	time1, err := time.Parse(time.RFC3339, ts1)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid first timestamp: %w", err))
	}

	time2, err := time.Parse(time.RFC3339, ts2)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid second timestamp: %w", err))
	}

	return types.Bool(time1.Before(time2))
}

func (t *TimeLib) time_after(t1, t2 ref.Val) ref.Val {
	ts1, ok := t1.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("first timestamp must be a string"))
	}
	ts2, ok := t2.Value().(string)
	if !ok {
		return types.WrapErr(fmt.Errorf("second timestamp must be a string"))
	}

	time1, err := time.Parse(time.RFC3339, ts1)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid first timestamp: %w", err))
	}

	time2, err := time.Parse(time.RFC3339, ts2)
	if err != nil {
		return types.WrapErr(fmt.Errorf("invalid second timestamp: %w", err))
	}

	return types.Bool(time1.After(time2))
}
