package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestEqualHandler_Evaluate(t *testing.T) {
	log := logr.Discard()
	handler := NewEqualHandler(log, nil)

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		// Bool comparisons
		{
			name:     "bool equal true",
			key:      true,
			value:    true,
			expected: true,
		},
		{
			name:     "bool equal false",
			key:      false,
			value:    false,
			expected: true,
		},
		{
			name:     "bool not equal",
			key:      true,
			value:    false,
			expected: false,
		},
		{
			name:     "bool with wrong type",
			key:      true,
			value:    "true",
			expected: false,
		},
		// Int comparisons
		{
			name:     "int equal",
			key:      42,
			value:    42,
			expected: true,
		},
		{
			name:     "int not equal",
			key:      42,
			value:    43,
			expected: false,
		},
		{
			name:     "int64 equal",
			key:      int64(100),
			value:    int64(100),
			expected: true,
		},
		{
			name:     "int with int64",
			key:      42,
			value:    int64(42),
			expected: true,
		},
		{
			name:     "int with float64 no fraction",
			key:      42,
			value:    float64(42),
			expected: true,
		},
		{
			name:     "int with float64 with fraction",
			key:      42,
			value:    42.5,
			expected: false,
		},
		{
			name:     "int with string",
			key:      42,
			value:    "42",
			expected: true,
		},
		{
			name:     "int with invalid string",
			key:      42,
			value:    "invalid",
			expected: false,
		},
		// Float comparisons
		{
			name:     "float equal",
			key:      3.14,
			value:    3.14,
			expected: true,
		},
		{
			name:     "float not equal",
			key:      3.14,
			value:    2.71,
			expected: false,
		},
		{
			name:     "float with int no fraction",
			key:      float64(42),
			value:    42,
			expected: true,
		},
		{
			name:     "float with int64",
			key:      float64(100),
			value:    int64(100),
			expected: true,
		},
		{
			name:     "float with string",
			key:      3.14,
			value:    "3.14",
			expected: true,
		},
		{
			name:     "float with invalid string",
			key:      3.14,
			value:    "invalid",
			expected: false,
		},
		// String comparisons
		{
			name:     "string equal",
			key:      "hello",
			value:    "hello",
			expected: true,
		},
		{
			name:     "string not equal",
			key:      "hello",
			value:    "world",
			expected: false,
		},
		{
			name:     "string with wildcard match",
			key:      "hello-world",
			value:    "hello-*",
			expected: true,
		},
		{
			name:     "string with wildcard no match",
			key:      "goodbye-world",
			value:    "hello-*",
			expected: false,
		},
		{
			name:     "string with wrong type",
			key:      "hello",
			value:    123,
			expected: false,
		},
		// Resource quantity comparisons
		{
			name:     "resource quantity equal",
			key:      "100Mi",
			value:    "100Mi",
			expected: true,
		},
		{
			name:     "resource quantity equal different format",
			key:      "1Gi",
			value:    "1024Mi",
			expected: true,
		},
		{
			name:     "resource quantity not equal",
			key:      "100Mi",
			value:    "200Mi",
			expected: false,
		},
		// Duration comparisons
		{
			name:     "duration equal",
			key:      "1h",
			value:    "60m",
			expected: true,
		},
		{
			name:     "duration not equal",
			key:      "1h",
			value:    "30m",
			expected: false,
		},
		// Map comparisons
		{
			name: "map equal",
			key: map[string]interface{}{
				"foo": "bar",
			},
			value: map[string]interface{}{
				"foo": "bar",
			},
			expected: true,
		},
		{
			name: "map not equal",
			key: map[string]interface{}{
				"foo": "bar",
			},
			value: map[string]interface{}{
				"foo": "baz",
			},
			expected: false,
		},
		{
			name: "map with wrong type",
			key: map[string]interface{}{
				"foo": "bar",
			},
			value:    "not a map",
			expected: false,
		},
		// Slice comparisons
		{
			name:     "slice equal",
			key:      []interface{}{"a", "b", "c"},
			value:    []interface{}{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "slice not equal",
			key:      []interface{}{"a", "b", "c"},
			value:    []interface{}{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "slice different length",
			key:      []interface{}{"a", "b", "c"},
			value:    []interface{}{"a", "b"},
			expected: false,
		},
		{
			name:     "slice with wrong type",
			key:      []interface{}{"a", "b", "c"},
			value:    "not a slice",
			expected: false,
		},
		// Unsupported type
		{
			name:     "unsupported type",
			key:      struct{}{},
			value:    struct{}{},
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

func TestEqualHandler_ValidateValueWithIntPattern(t *testing.T) {
	log := logr.Discard()
	handler := EqualHandler{log: log}

	tests := []struct {
		name     string
		key      int64
		value    interface{}
		expected bool
	}{
		{
			name:     "int with int",
			key:      int64(10),
			value:    10,
			expected: true,
		},
		{
			name:     "int with int64",
			key:      int64(10),
			value:    int64(10),
			expected: true,
		},
		{
			name:     "int with float64 no fraction",
			key:      int64(10),
			value:    float64(10),
			expected: true,
		},
		{
			name:     "int with float64 with fraction",
			key:      int64(10),
			value:    10.5,
			expected: false,
		},
		{
			name:     "int with string valid",
			key:      int64(10),
			value:    "10",
			expected: true,
		},
		{
			name:     "int with string invalid",
			key:      int64(10),
			value:    "abc",
			expected: false,
		},
		{
			name:     "int with wrong type",
			key:      int64(10),
			value:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithIntPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEqualHandler_ValidateValueWithFloatPattern(t *testing.T) {
	log := logr.Discard()
	handler := EqualHandler{log: log}

	tests := []struct {
		name     string
		key      float64
		value    interface{}
		expected bool
	}{
		{
			name:     "float with int",
			key:      float64(10),
			value:    10,
			expected: true,
		},
		{
			name:     "float with int64",
			key:      float64(10),
			value:    int64(10),
			expected: true,
		},
		{
			name:     "float with fraction and int",
			key:      10.5,
			value:    10,
			expected: false,
		},
		{
			name:     "float with float64",
			key:      3.14,
			value:    3.14,
			expected: true,
		},
		{
			name:     "float with string valid",
			key:      3.14,
			value:    "3.14",
			expected: true,
		},
		{
			name:     "float with string invalid",
			key:      3.14,
			value:    "abc",
			expected: false,
		},
		{
			name:     "float with wrong type",
			key:      3.14,
			value:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithFloatPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEqualHandler_ValidateValueWithBoolPattern(t *testing.T) {
	log := logr.Discard()
	handler := EqualHandler{log: log}

	tests := []struct {
		name     string
		key      bool
		value    interface{}
		expected bool
	}{
		{
			name:     "true equals true",
			key:      true,
			value:    true,
			expected: true,
		},
		{
			name:     "false equals false",
			key:      false,
			value:    false,
			expected: true,
		},
		{
			name:     "true not equals false",
			key:      true,
			value:    false,
			expected: false,
		},
		{
			name:     "bool with wrong type",
			key:      true,
			value:    "true",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithBoolPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}
