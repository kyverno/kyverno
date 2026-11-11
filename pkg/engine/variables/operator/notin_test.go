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
		// A request-supplied key of "*" is matched literally, so it is
		// absent from a concrete policy allow-list and NotIn reports true.
		{
			name:     "request key wildcard is matched literally against the policy allow-list",
			key:      "*",
			value:    []interface{}{"apple", "banana"},
			expected: true,
		},
		// Invariant: an admin value-side wildcard still matches a
		// concrete request key, so NotIn is false.
		{
			name:     "admin value wildcard still matches concrete key",
			key:      "prod-web",
			value:    []interface{}{"prod-*"},
			expected: false,
		},
		// Invariant: an admin universal catch-all still matches any key.
		{
			name:     "admin universal catch-all still matches any concrete key",
			key:      "anything",
			value:    []interface{}{"*"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}
