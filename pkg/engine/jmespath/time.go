package jmespath

import (
	"reflect"
	"strconv"
	"time"
)

// function names
var (
	timeSince    = "time_since"
	timeNow      = "time_now"
	timeNowUtc   = "time_now_utc"
	timeAdd      = "time_add"
	timeParse    = "time_parse"
	timeToCron   = "time_to_cron"
	timeUtc      = "time_utc"
	timeDiff     = "time_diff"
	timeBefore   = "time_before"
	timeAfter    = "time_after"
	timeBetween  = "time_between"
	timeTruncate = "time_truncate"
)

func getTimeArg(f string, arguments []interface{}, index int) (time.Time, error) {
	var empty time.Time
	arg, err := validateArg(f, arguments, index, reflect.String)
	if err != nil {
		return empty, err
	}
	return time.Parse(time.RFC3339, arg.String())
}

func getDurationArg(f string, arguments []interface{}, index int) (time.Duration, error) {
	var empty time.Duration
	arg, err := validateArg(f, arguments, index, reflect.String)
	if err != nil {
		return empty, err
	}
	return time.ParseDuration(arg.String())
}

func jpTimeSince(arguments []interface{}) (interface{}, error) {
	var err error
	layout, err := validateArg(timeSince, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}
	ts1, err := validateArg(timeSince, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}
	ts2, err := validateArg(timeSince, arguments, 2, reflect.String)
	if err != nil {
		return nil, err
	}
	var t1, t2 time.Time
	if layout.String() != "" {
		t1, err = time.Parse(layout.String(), ts1.String())
	} else {
		t1, err = time.Parse(time.RFC3339, ts1.String())
	}
	if err != nil {
		return nil, err
	}
	t2 = time.Now()
	if ts2.String() != "" {
		if layout.String() != "" {
			t2, err = time.Parse(layout.String(), ts2.String())
		} else {
			t2, err = time.Parse(time.RFC3339, ts2.String())
		}
		if err != nil {
			return nil, err
		}
	}
	return t2.Sub(t1).String(), nil
}

func jpTimeNow(arguments []interface{}) (interface{}, error) {
	return time.Now().Format(time.RFC3339), nil
}

func jpTimeNowUtc(arguments []interface{}) (interface{}, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

func jpTimeToCron(arguments []interface{}) (interface{}, error) {
	if t, err := getTimeArg(timeToCron, arguments, 0); err != nil {
		return nil, err
	} else {
		var cron string = ""
		cron += strconv.Itoa(t.Minute()) + " "
		cron += strconv.Itoa(t.Hour()) + " "
		cron += strconv.Itoa(t.Day()) + " "
		cron += strconv.Itoa(int(t.Month())) + " "
		cron += strconv.Itoa(int(t.Weekday()))
		return cron, nil
	}
}

func jpTimeAdd(arguments []interface{}) (interface{}, error) {
	if t, err := getTimeArg(timeToCron, arguments, 0); err != nil {
		return nil, err
	} else if d, err := getDurationArg(timeToCron, arguments, 1); err != nil {
		return nil, err
	} else {
		return t.Add(d).Format(time.RFC3339), nil
	}
}

func jpTimeParse(arguments []interface{}) (interface{}, error) {
	var err error
	layout, err := validateArg(timeParse, arguments, 0, reflect.String)
	if err != nil {
		return nil, err
	}
	ts, err := validateArg(timeParse, arguments, 1, reflect.String)
	if err != nil {
		return nil, err
	}
	_, err = strconv.ParseInt(layout.String(), 10, 64)
	if err == nil { // epoch time layout
		epochTime, err := strconv.ParseInt(ts.String(), 10, 64)
		if err != nil {
			return nil, err
		}
		return time.Unix(epochTime, 0).UTC().Format(time.RFC3339), nil
	}
	t, err := time.Parse(layout.String(), ts.String())
	if err != nil {
		return nil, err
	}
	return t.Format(time.RFC3339), nil
}

func jpTimeUtc(arguments []interface{}) (interface{}, error) {
	if t, err := getTimeArg(timeUtc, arguments, 0); err != nil {
		return nil, err
	} else {
		return t.UTC().Format(time.RFC3339), nil
	}
}

func jpTimeDiff(arguments []interface{}) (interface{}, error) {
	if t1, err := getTimeArg(timeDiff, arguments, 0); err != nil {
		return nil, err
	} else if t2, err := getTimeArg(timeDiff, arguments, 1); err != nil {
		return nil, err
	} else {
		return t2.Sub(t1).String(), nil
	}
}

func jpTimeBefore(arguments []interface{}) (interface{}, error) {
	if t1, err := getTimeArg(timeBefore, arguments, 0); err != nil {
		return nil, err
	} else if t2, err := getTimeArg(timeBefore, arguments, 1); err != nil {
		return nil, err
	} else {
		return t1.Before(t2), nil
	}
}

func jpTimeAfter(arguments []interface{}) (interface{}, error) {
	if t1, err := getTimeArg(timeAfter, arguments, 0); err != nil {
		return nil, err
	} else if t2, err := getTimeArg(timeAfter, arguments, 1); err != nil {
		return nil, err
	} else {
		return t1.After(t2), nil
	}
}

func jpTimeBetween(arguments []interface{}) (interface{}, error) {
	if t, err := getTimeArg(timeBetween, arguments, 0); err != nil {
		return nil, err
	} else if start, err := getTimeArg(timeBetween, arguments, 1); err != nil {
		return nil, err
	} else if end, err := getTimeArg(timeBetween, arguments, 2); err != nil {
		return nil, err
	} else {
		return t.After(start) && t.Before(end), nil
	}
}

func jpTimeTruncate(arguments []interface{}) (interface{}, error) {
	if t, err := getTimeArg(timeTruncate, arguments, 0); err != nil {
		return nil, err
	} else if d, err := getDurationArg(timeTruncate, arguments, 1); err != nil {
		return nil, err
	} else {
		return t.Truncate(d).Format(time.RFC3339), nil
	}
}
