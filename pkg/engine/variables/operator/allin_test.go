package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestAllInHandler_Evaluate(t *testing.T) {
	log := logr.Discard()
	handler := NewAllInHandler(log, nil)

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
			name:     "key is array and all in value with wildcard",
			key:      []interface{}{"kube-system", "kube-public"},
			value:    "kube-*",
			expected: true,
		},
		{
			name:     "key is array and partially in value",
			key:      []interface{}{"kube-system", "default"},
			value:    "kube-system",
			expected: false,
		},
		{
			name:     "key is array and not in value",
			key:      []interface{}{"default", "kyverno"},
			value:    "kube-*",
			expected: false,
		},
		{
			name:     "key and value are array and all in value",
			key:      []interface{}{"kube-system", "kube-public"},
			value:    []interface{}{"kube-*", "ky*"},
			expected: true,
		},
		{
			name:     "key and value are array and not all in value",
			key:      []interface{}{"default", "kyverno"},
			value:    []interface{}{"kube-*", "kube-system"},
			expected: false,
		},
		{
			name:     "unsupported key type",
			key:      map[string]interface{}{"foo": "bar"},
			value:    "test",
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
			name:     "mixed list request key, wildcard element breaks the all-match conjunction",
			key:      []interface{}{"*", "apple"},
			value:    []interface{}{"apple", "banana"},
			expected: false,
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
		// Exact-membership positive/negative.
		{
			name:     "scalar key: exact membership match",
			key:      "apple",
			value:    []interface{}{"apple", "banana"},
			expected: true,
		},
		{
			name:     "scalar key: exact membership no match",
			key:      "cherry",
			value:    []interface{}{"apple", "banana"},
			expected: false,
		},
		{
			name:     "list key: exact membership match",
			key:      []interface{}{"apple", "banana"},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "list key: exact membership no match",
			key:      []interface{}{"apple", "cherry"},
			value:    []interface{}{"apple", "banana"},
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
