package jmespath

import (
	"math"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
)

func Test_IfaceToStringPrecision(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input any
		want  string
	}{
		{"float64 boundary", float64(16777217), "16777217"},
		{"float64 large", float64(1000000007), "1000000007"},
		{"float64 negative", float64(-16777217), "-16777217"},
		{"float64 fraction", 1.0000000000000002, "1.0000000000000002"},
		{"float64 zero", float64(0), "0"},
		{"float32", float32(1.2), "1.2"},
		{"integer", 42, "42"},
		{"string", "value", "value"},
		{"boolean", true, "true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ifaceToString(tc.input)
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()
		_, err := ifaceToString(nil)
		assert.ErrorContains(t, err, "undefined type cast")
	})
}

func Test_NumericFunctionsPrecision(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		query string
		want  any
	}{
		{"regex_match('^16777217$', value)", true},
		{"regex_match('^16777216$', value)", false},
		{"pattern_match('16777217', value)", true},
		{"pattern_match('16777216', value)", false},
		{"regex_replace_all('^16777217$', value, 'matched')", "matched"},
		{"object_from_lists([value], ['matched'])", map[string]any{"16777217": "matched"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			query, err := jmespathInterface.Query(tc.query)
			assert.NilError(t, err)
			got, err := query.Search(map[string]any{"value": float64(16777217)})
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tc.want)
		})
	}
}

func Fuzz_IfaceToStringFloat64RoundTrip(f *testing.F) {
	for _, value := range []float64{0, math.Copysign(0, -1), 16777217, -16777217, 1000000007, 1.0000000000000002, math.SmallestNonzeroFloat64, math.MaxFloat64} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value float64) {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Skip("only finite numbers occur in JSON")
		}
		encoded, err := ifaceToString(value)
		assert.NilError(t, err)
		decoded, err := strconv.ParseFloat(encoded, 64)
		assert.NilError(t, err)
		assert.Equal(t, math.Float64bits(decoded), math.Float64bits(value))
	})
}
