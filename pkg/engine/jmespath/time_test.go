package jmespath

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"gotest.tools/assert"
)

func Test_TimeSince(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_since('', '2021-01-02T15:04:05-07:00', '2021-01-10T03:14:05-07:00')",
			expectedResult: "180h10m0s",
		},
		{
			test:           "time_since('Mon Jan _2 15:04:05 MST 2006', 'Mon Jan 02 15:04:05 MST 2021', 'Mon Jan 10 03:14:16 MST 2021')",
			expectedResult: "180h10m11s",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_TimeToCron(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_to_cron('2023-02-02T15:04:05Z')",
			expectedResult: "4 15 2 2 4",
		},
		{
			test:           "time_to_cron(time_utc('2023-02-02T15:04:05-07:00'))",
			expectedResult: "4 22 2 2 4",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_TimeAdd(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_add('2021-01-02T15:04:05-07:00', '3h')",
			expectedResult: "2021-01-02T18:04:05-07:00",
		},
		{
			test:           "time_add(time_parse('Mon Jan 02 15:04:05 MST 2006', 'Sat Jan 02 15:04:05 MST 2021'), '5h30m40s')",
			expectedResult: "2021-01-02T20:34:45Z",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_TimeParse(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_parse('2006-01-02T15:04:05Z07:00', '2021-01-02T15:04:05-07:00')",
			expectedResult: "2021-01-02T15:04:05-07:00",
		},
		{
			test:           "time_parse('Mon Jan 02 15:04:05 MST 2006', 'Sat Jan 02 15:04:05 MST 2021')",
			expectedResult: "2021-01-02T15:04:05Z",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_TimeUtc(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_utc('2023-02-02T15:04:05Z')",
			expectedResult: "2023-02-02T15:04:05Z",
		},
		{
			test:           "time_utc('2023-02-02T15:04:05-07:00')",
			expectedResult: "2023-02-02T22:04:05Z",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_TimeDiff(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_diff('2021-01-02T15:04:05-07:00', '2021-01-10T03:14:05-07:00')",
			expectedResult: "180h10m0s",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_getTimeArg(t *testing.T) {
	mustParse := func(s string) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			panic(err)
		}
		return t
	}
	type args struct {
		f         string
		arguments []interface{}
		index     int
	}
	tests := []struct {
		name    string
		args    args
		want    time.Time
		wantErr bool
	}{{
		args: args{
			f: "test",
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
			},
			index: 0,
		},
		want: mustParse("2021-01-02T15:04:05-07:00"),
	}, {
		args: args{
			f: "test",
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
			},
			index: 1,
		},
		wantErr: true,
	}, {
		args: args{
			f: "test",
			arguments: []interface{}{
				1,
			},
			index: 0,
		},
		wantErr: true,
	}, {
		args: args{
			f: "test",
			arguments: []interface{}{
				"",
			},
			index: 0,
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTimeArg(tt.args.f, tt.args.arguments, tt.args.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("getTimeArg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTimeArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDurationArg(t *testing.T) {
	mustParse := func(s string) time.Duration {
		t, err := time.ParseDuration(s)
		if err != nil {
			panic(err)
		}
		return t
	}
	type args struct {
		f         string
		arguments []interface{}
		index     int
	}
	tests := []struct {
		name    string
		args    args
		want    time.Duration
		wantErr bool
	}{{
		args: args{
			f: "test",
			arguments: []interface{}{
				"20s",
			},
			index: 0,
		},
		want: mustParse("20s"),
	}, {
		args: args{
			f: "test",
			arguments: []interface{}{
				"20s",
			},
			index: 1,
		},
		wantErr: true,
	}, {
		args: args{
			f: "test",
			arguments: []interface{}{
				1,
			},
			index: 0,
		},
		wantErr: true,
	}, {
		args: args{
			f: "test",
			arguments: []interface{}{
				"",
			},
			index: 0,
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDurationArg(tt.args.f, tt.args.arguments, tt.args.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDurationArg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getDurationArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpTimeBefore(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T16:04:05-07:00",
			},
		},
		want: true,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
			},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T16:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
			},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{
				1,
				"2021-01-02T15:04:05-07:00",
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				1,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpTimeBefore(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpTimeBefore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpTimeBefore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpTimeAfter(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T16:04:05-07:00",
			},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
			},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T16:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
			},
		},
		want: true,
	}, {
		args: args{
			arguments: []interface{}{
				1,
				"2021-01-02T15:04:05-07:00",
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				1,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpTimeAfter(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpTimeAfter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpTimeAfter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpTimeBetween(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"2021-01-02T17:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T18:04:05-07:00",
			},
		},
		want: true,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T18:04:05-07:00",
			},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T18:04:05-07:00",
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T18:04:05-07:00",
			},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{
				1,
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T18:04:05-07:00",
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				1,
				"2021-01-02T18:04:05-07:00",
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"2021-01-02T15:04:05-07:00",
				"2021-01-02T18:04:05-07:00",
				1,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpTimeBetween(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpTimeBetween() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpTimeBetween() = %v, want %v", got, tt.want)
			}
		})
	}
}
