package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestAnyInHandler_Evaluate(t *testing.T) {
	log := logr.Discard()
	handler := NewAnyInHandler(log, nil)

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "key is string and in value",
			key:      "kyverno",
			value:    "kyverno",
			expected: true,
		},
		{
			name:     "key is string and in value with wildcard",
			key:      "kube-system",
			value:    []interface{}{"default", "kube-*"},
			expected: true,
		},
		{
			name:     "key is string and not in value",
			key:      "kyverno",
			value:    "default",
			expected: false,
		},
		{
			name:     "key is int and in value",
			key:      64,
			value:    "64",
			expected: true,
		},
		{
			name:     "key is int and in value slice",
			key:      1,
			value:    []interface{}{1, 2, 3},
			expected: true,
		},
		{
			name:     "key is int and not in value",
			key:      64,
			value:    "default",
			expected: false,
		},
		{
			name:     "key is float and in value",
			key:      3.14,
			value:    "3.14",
			expected: true,
		},
		{
			name:     "key is float and in value slice",
			key:      2.2,
			value:    []interface{}{1.1, 2.2, 3.3},
			expected: true,
		},
		{
			name:     "key is float and not in value",
			key:      3.14,
			value:    "default",
			expected: false,
		},
		{
			name:     "key is boolean and in value",
			key:      true,
			value:    "true",
			expected: true,
		},
		{
			name:     "key is array and all in value with wildcard",
			key:      []interface{}{"kube-system", "kube-public"},
			value:    "kube-*",
			expected: true,
		},
		{
			name:     "key is array and partially in value",
			key:      []interface{}{"kube-system", "default"},
			value:    "kube-system",
			expected: true,
		},
		{
			name:     "key is array and not in value",
			key:      []interface{}{"default", "kyverno"},
			value:    "kube-*",
			expected: false,
		},
		{
			name:     "key and value are array and any in value",
			key:      []interface{}{"default", "kyverno"},
			value:    []interface{}{"kube-*", "ky*"},
			expected: true,
		},
		{
			name:     "key and value are array and none in value",
			key:      []interface{}{"default", "kyverno"},
			value:    []interface{}{"kube-*", "kube-system"},
			expected: false,
		},
		{
			name:     "key is an empty array",
			key:      []interface{}{},
			value:    []interface{}{"default", "kyverno"},
			expected: false,
		},
		{
			name:     "key is an empty string and value is an array",
			key:      "",
			value:    []interface{}{"default", "kyverno"},
			expected: false,
		},
		{
			name:     "unsupported key type",
			key:      map[string]interface{}{"foo": "bar"},
			value:    "test",
			expected: false,
		},
		// Range pattern coverage (handleRange path)
		{
			name:     "key is string and in numeric range",
			key:      "5",
			value:    "1-10",
			expected: true,
		},
		{
			name:     "key is string and outside numeric range",
			key:      "0",
			value:    "1-10",
			expected: false,
		},
		{
			name:     "key is array and any value in numeric range",
			key:      []interface{}{"0", "5"},
			value:    "1-10",
			expected: true,
		},
		{
			name:     "key is array and no value in numeric range",
			key:      []interface{}{"0", "20"},
			value:    "1-10",
			expected: false,
		},
		// JSON array string value coverage
		{
			name:     "key is string and in JSON array string",
			key:      "apple",
			value:    `["apple", "banana"]`,
			expected: true,
		},
		{
			name:     "key is string and not in JSON array string",
			key:      "cherry",
			value:    `["apple", "banana"]`,
			expected: false,
		},
		{
			name:     "key is array and any in JSON array string",
			key:      []interface{}{"apple", "cherry"},
			value:    `["apple", "banana"]`,
			expected: true,
		},
		{
			name:     "key is array and none in JSON array string",
			key:      []interface{}{"cherry", "date"},
			value:    `["apple", "banana"]`,
			expected: false,
		},
		// A request-supplied key of "*" is matched literally and must never
		// act as a pattern against a concrete policy allow-list, in either
		// scalar- or list-key form.
		{
			name:     "scalar request key wildcard is matched literally against the policy allow-list",
			key:      "*",
			value:    []interface{}{"apple", "banana"},
			expected: false,
		},
		{
			name:     "list request key wildcard is matched literally against the policy allow-list",
			key:      []interface{}{"*"},
			value:    []interface{}{"apple", "banana"},
			expected: false,
		},
		{
			name:     "mixed list request key, wildcard element is matched literally but concrete element still matches",
			key:      []interface{}{"*", "apple"},
			value:    []interface{}{"apple", "banana"},
			expected: true,
		},
		// Invariant: an admin value-side wildcard still matches a
		// concrete request key, in both forms.
		{
			name:     "scalar key: admin value wildcard still matches concrete key",
			key:      "prod-web",
			value:    []interface{}{"prod-*"},
			expected: true,
		},
		{
			name:     "list key: admin value wildcard still matches concrete key",
			key:      []interface{}{"prod-web"},
			value:    []interface{}{"prod-*"},
			expected: true,
		},
		// Invariant: an admin universal catch-all still matches any key,
		// including an opaque one (nested object) in list-key form.
		{
			name:     "scalar key: admin universal catch-all still matches any concrete key",
			key:      "anything",
			value:    []interface{}{"*"},
			expected: true,
		},
		{
			name:     "list key: admin universal catch-all still matches an opaque key element",
			key:      []interface{}{map[string]interface{}{"foo": "bar"}},
			value:    []interface{}{"*"},
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
