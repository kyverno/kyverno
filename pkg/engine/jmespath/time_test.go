package jmespath

import (
	"fmt"
	"testing"

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
			query, err := New(tc.test)
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
			query, err := New(tc.test)
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
			query, err := New(tc.test)
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
			query, err := New(tc.test)
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
			query, err := New(tc.test)
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
			query, err := New(tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}
