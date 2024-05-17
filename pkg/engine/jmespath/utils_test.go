package jmespath

import (
	"fmt"
	"math"
	"testing"

	"gotest.tools/assert"
)

func Test_intNumber(t *testing.T) {
	testCases := []struct {
		number         float64
		expectedResult int
	}{
		{
			number:         0.0,
			expectedResult: 0,
		},
		{
			number:         1.0,
			expectedResult: 1,
		},
		{
			number:         -1.0,
			expectedResult: -1,
		},
		{
			number:         math.MaxInt32,
			expectedResult: math.MaxInt32,
		},
		{
			number:         math.MinInt32,
			expectedResult: math.MinInt32,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			result, resultErr := intNumber(tc.number)

			assert.NilError(t, resultErr)
			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_intNumber_Error(t *testing.T) {
	testCases := []struct {
		number      float64
		expectedMsg string
	}{
		{
			number:      1.5,
			expectedMsg: `expected an integer number but got: 1.5`,
		},
		{
			number:      math.NaN(),
			expectedMsg: `expected an integer number but got: NaN`,
		},
		{
			number:      math.Inf(1),
			expectedMsg: `expected an integer number but got: +Inf`,
		},
		{
			number:      math.Inf(-1),
			expectedMsg: `expected an integer number but got: -Inf`,
		},
		{
			number:      math.MaxFloat64,
			expectedMsg: `number is outside the range of integer numbers: 1.7976931348623157e+308`,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			_, resultErr := intNumber(tc.number)

			assert.Error(t, resultErr, tc.expectedMsg)
		})
	}
}
