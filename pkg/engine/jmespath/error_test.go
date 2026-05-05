package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		function string
		values   []interface{}
		want     string
	}{
		// Basic error formatting
		{"simple error", errorPrefix, "myFunc", nil, "JMESPath function 'myFunc': "},
		{"generic error", genericError, "compare", []interface{}{"values cannot be nil"}, "JMESPath function 'compare': values cannot be nil"},

		// Invalid argument type errors
		{"invalid arg type", invalidArgumentTypeError, "sum", []interface{}{1, "number"}, "JMESPath function 'sum': argument #1 is not of type number"},
		{"invalid arg type 2", invalidArgumentTypeError, "concat", []interface{}{2, "string"}, "JMESPath function 'concat': argument #2 is not of type string"},

		// Out of bounds errors
		{"out of bounds", argOutOfBoundsError, "slice", []interface{}{5, 3}, "JMESPath function 'slice': 5 argument is out of bounds (3)"},
		{"out of bounds zero", argOutOfBoundsError, "at", []interface{}{0, 0}, "JMESPath function 'at': 0 argument is out of bounds (0)"},

		// Specific error types
		{"zero division", zeroDivisionError, "divide", nil, "JMESPath function 'divide': Zero divisor passed"},
		{"non int modulo", nonIntModuloError, "modulo", nil, "JMESPath function 'modulo': Non-integer argument(s) passed for modulo"},
		{"type mismatch", typeMismatchError, "equals", nil, "JMESPath function 'equals': Types mismatch"},
		{"non int round", nonIntRoundError, "round", nil, "JMESPath function 'round': Non-integer argument(s) passed for round off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := formatError(tt.format, tt.function, tt.values...)
			assert.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Verify error constants contain expected patterns
	assert.Contains(t, errorPrefix, "JMESPath function")
	assert.Contains(t, invalidArgumentTypeError, "argument")
	assert.Contains(t, invalidArgumentTypeError, "not of type")
	assert.Contains(t, genericError, "%s")
	assert.Contains(t, argOutOfBoundsError, "out of bounds")
	assert.Contains(t, zeroDivisionError, "Zero divisor")
	assert.Contains(t, nonIntModuloError, "modulo")
	assert.Contains(t, typeMismatchError, "mismatch")
	assert.Contains(t, nonIntRoundError, "round")
}
