package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestNotEqualHandler_Evaluate(t *testing.T) {
	log := logr.Discard()
	handler := NewNotEqualHandler(log, nil)

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "bool",
			key:      true,
			value:    false,
			expected: true,
		},
		{
			name:     "int",
			key:      42,
			value:    43,
			expected: true,
		},
		{
			name:     "int vs fractional float",
			key:      int64(5),
			value:    float64(5.5),
			expected: true,
		},
		{
			name:     "float",
			key:      3.14,
			value:    2.71,
			expected: true,
		},
		{
			name:     "string",
			key:      "hello",
			value:    "world",
			expected: true,
		},
		{
			name:     "resource quantity vs non-quantity",
			key:      "100Mi",
			value:    "not-a-quantity",
			expected: true,
		},
		{
			name:     "duration",
			key:      "1h",
			value:    "30m",
			expected: true,
		},
		{
			name:     "map",
			key:      map[string]interface{}{"foo": "bar"},
			value:    map[string]interface{}{"foo": "baz"},
			expected: true,
		},
		{
			name:     "slice",
			key:      []interface{}{"a", "b"},
			value:    []interface{}{"a", "c"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_NotEqual_ResourceQuantityCrossType(t *testing.T) {
	handler := NewNotEqualHandler(logr.Discard(), nil)

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "string quantity equals int",
			key:      "5",
			value:    5,
			expected: false,
		},
		{
			name:     "string quantity equals int64",
			key:      "5",
			value:    int64(5),
			expected: false,
		},
		{
			name:     "string quantity equals float64",
			key:      "5",
			value:    float64(5),
			expected: false,
		},
		{
			name:     "different quantity",
			key:      "5",
			value:    6,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, handler.Evaluate(tt.key, tt.value))
		})
	}
}
