package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestNotInHandler_Evaluate(t *testing.T) {
	log := logr.Discard()
	handler := NewNotInHandler(log, nil)

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "string not in slice - true",
			key:      "grape",
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "string not in slice - false",
			key:      "apple",
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "slice subset of slice - all found so NotIn is false",
			key:      []interface{}{"apple", "banana"},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "slice subset of slice - not all found so NotIn is true",
			key:      []interface{}{"apple", "grape"},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: true,
		},
		// Non-string key elements (panic-safety)
		{
			name:     "numeric key element coerces like a scalar numeric key",
			key:      []interface{}{1, 2},
			value:    []interface{}{"1", "2", "3"},
			expected: false,
		},
		{
			name:     "opaque key element does not panic and witnesses NotIn",
			key:      []interface{}{"apple", map[string]interface{}{"foo": "bar"}},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "opaque key element alone does not panic and witnesses NotIn",
			key:      []interface{}{[]interface{}{"nested"}},
			value:    []interface{}{"apple", "banana", "cherry"},
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
