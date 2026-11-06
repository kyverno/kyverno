package time

import (
	"fmt"
	gotime "time"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func time_now() ref.Val {
	return types.String(gotime.Now().Format(gotime.RFC3339))
}

func time_now_utc() ref.Val {
	return types.String(gotime.Now().UTC().Format(gotime.RFC3339))
}

func time_parse(layout ref.Val, value ref.Val) ref.Val {
	l, ok := layout.(types.String)
	if !ok {
		return types.NewErr("time_parse: layout must be a string")
	}
	v, ok := value.(types.String)
	if !ok {
		return types.NewErr("time_parse: value must be a string")
	}
	t, err := gotime.Parse(string(l), string(v))
	if err != nil {
		return types.WrapErr(err)
	}
	return types.String(t.Format(gotime.RFC3339))
}

func time_add(ts ref.Val, duration ref.Val) ref.Val {
	t, err := parseRFC3339(ts)
	if err != nil {
		return types.WrapErr(err)
	}
	d, ok := duration.(types.String)
	if !ok {
		return types.NewErr("time_add: duration must be a string")
	}
	dur, err := gotime.ParseDuration(string(d))
	if err != nil {
		return types.WrapErr(err)
	}
	return types.String(t.Add(dur).Format(gotime.RFC3339))
}

func time_diff(t1 ref.Val, t2 ref.Val) ref.Val {
	t1t, err := parseRFC3339(t1)
	if err != nil {
		return types.WrapErr(err)
	}
	t2t, err := parseRFC3339(t2)
	if err != nil {
		return types.WrapErr(err)
	}
	return types.String(t2t.Sub(t1t).String())
}

func time_before(t1 ref.Val, t2 ref.Val) ref.Val {
	t1t, err := parseRFC3339(t1)
	if err != nil {
		return types.WrapErr(err)
	}
	t2t, err := parseRFC3339(t2)
	if err != nil {
		return types.WrapErr(err)
	}
	return types.Bool(t1t.Before(t2t))
}

func time_after(t1 ref.Val, t2 ref.Val) ref.Val {
	t1t, err := parseRFC3339(t1)
	if err != nil {
		return types.WrapErr(err)
	}
	t2t, err := parseRFC3339(t2)
	if err != nil {
		return types.WrapErr(err)
	}
	return types.Bool(t1t.After(t2t))
}

// parseRFC3339 is a shared helper used by all functions that accept timestamp arguments
func parseRFC3339(val ref.Val) (gotime.Time, error) {
	s, ok := val.(types.String)
	if !ok {
		return gotime.Time{}, fmt.Errorf("expected string, got %T", val)
	}
	return gotime.Parse(gotime.RFC3339, string(s))
}