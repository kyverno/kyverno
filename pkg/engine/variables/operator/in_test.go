package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestInHandler_Evaluate(t *testing.T) {
	log := logr.Discard()
	handler := NewInHandler(log, nil)

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		// String key with slice value
		{
			name:     "string in slice - found",
			key:      "apple",
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "string in slice - not found",
			key:      "grape",
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "string in slice - empty slice",
			key:      "apple",
			value:    []interface{}{},
			expected: false,
		},
		// String key with string value (wildcard)
		{
			name:     "string matches string value",
			key:      "hello",
			value:    "hello",
			expected: true,
		},
		{
			name:     "string matches wildcard value",
			key:      "hello-world",
			value:    "hello-*",
			expected: true,
		},
		{
			name:     "string no match wildcard value",
			key:      "goodbye-world",
			value:    "hello-*",
			expected: false,
		},
		// String key with JSON array string
		{
			name:     "string in JSON array string - found",
			key:      "apple",
			value:    `["apple", "banana", "cherry"]`,
			expected: true,
		},
		{
			name:     "string in JSON array string - not found",
			key:      "grape",
			value:    `["apple", "banana", "cherry"]`,
			expected: false,
		},
		// Numeric types (converted to string)
		{
			name:     "int in slice - found",
			key:      42,
			value:    []interface{}{"42", "43", "44"},
			expected: true,
		},
		{
			name:     "int in slice - not found",
			key:      99,
			value:    []interface{}{"42", "43", "44"},
			expected: false,
		},
		{
			name:     "int64 in slice - found",
			key:      int64(100),
			value:    []interface{}{"100", "200"},
			expected: true,
		},
		{
			name:     "float64 in slice - found",
			key:      float64(3.14),
			value:    []interface{}{"3.14", "2.71"},
			expected: true,
		},
		{
			name:     "bool in slice - found",
			key:      true,
			value:    []interface{}{"true", "false"},
			expected: true,
		},
		// Slice key (set membership)
		{
			name:     "slice subset of slice - all found",
			key:      []interface{}{"apple", "banana"},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: true,
		},
		{
			name:     "slice subset of slice - not all found",
			key:      []interface{}{"apple", "grape"},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "slice subset of JSON array - all found",
			key:      []interface{}{"apple", "banana"},
			value:    `["apple", "banana", "cherry"]`,
			expected: true,
		},
		// Unsupported types
		{
			name:     "unsupported key type",
			key:      map[string]interface{}{"foo": "bar"},
			value:    []interface{}{"a", "b"},
			expected: false,
		},
		{
			name:     "invalid value type",
			key:      "apple",
			value:    12345,
			expected: false,
		},
		// Non-string key elements (panic-safety)
		{
			name:     "numeric key element coerces like a scalar numeric key",
			key:      []interface{}{1, 2},
			value:    []interface{}{"1", "2", "3"},
			expected: true,
		},
		{
			name:     "opaque key element does not panic and fails the conjunction",
			key:      []interface{}{"apple", map[string]interface{}{"foo": "bar"}},
			value:    []interface{}{"apple", "banana", "cherry"},
			expected: false,
		},
		{
			name:     "opaque key element alone does not panic and fails the conjunction",
			key:      []interface{}{[]interface{}{"nested"}},
			value:    []interface{}{"apple", "banana", "cherry"},
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

func TestInHandler_ValidateValueWithStringPattern(t *testing.T) {
	log := logr.Discard()
	handler := InHandler{log: log}

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected bool
	}{
		{
			name:     "found in slice",
			key:      "test",
			value:    []interface{}{"test", "other"},
			expected: true,
		},
		{
			name:     "not found in slice",
			key:      "missing",
			value:    []interface{}{"test", "other"},
			expected: false,
		},
		{
			name:     "matches string with wildcard",
			key:      "prefix-suffix",
			value:    "prefix-*",
			expected: true,
		},
		{
			name:     "invalid value type",
			key:      "test",
			value:    123,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithStringPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInHandler_ValidateValueWithElementSetPattern(t *testing.T) {
	log := logr.Discard()
	handler := InHandler{log: log}

	tests := []struct {
		name     string
		key      []interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "all keys in slice",
			key:      []interface{}{"a", "b"},
			value:    []interface{}{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "not all keys in slice",
			key:      []interface{}{"a", "d"},
			value:    []interface{}{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "all keys in JSON array",
			key:      []interface{}{"a", "b"},
			value:    `["a", "b", "c"]`,
			expected: true,
		},
		{
			name:     "single key matches string",
			key:      []interface{}{"test"},
			value:    "test",
			expected: true,
		},
		{
			name:     "single key no match string",
			key:      []interface{}{"other"},
			value:    "test",
			expected: false,
		},
		{
			name:     "invalid value type",
			key:      []interface{}{"a"},
			value:    123,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithElementSetPattern(toElements(tt.key), tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeyExistsInArray(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name           string
		key            string
		value          interface{}
		expectInvalid  bool
		expectKeyExist bool
	}{
		{
			name:           "key in slice",
			key:            "apple",
			value:          []interface{}{"apple", "banana"},
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "key not in slice",
			key:            "cherry",
			value:          []interface{}{"apple", "banana"},
			expectInvalid:  false,
			expectKeyExist: false,
		},
		{
			name:           "key matches with wildcard in slice",
			key:            "hello-world",
			value:          []interface{}{"hello-*", "goodbye"},
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "key is wildcard matches value",
			key:            "hello-*",
			value:          []interface{}{"hello-world", "goodbye"},
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "key matches string value",
			key:            "test",
			value:          "test",
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "key matches wildcard string",
			key:            "prefix-suffix",
			value:          "prefix-*",
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "key in JSON array",
			key:            "a",
			value:          `["a", "b", "c"]`,
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "key not in JSON array",
			key:            "d",
			value:          `["a", "b", "c"]`,
			expectInvalid:  false,
			expectKeyExist: false,
		},
		{
			name:           "invalid JSON",
			key:            "a",
			value:          `not-valid-json`,
			expectInvalid:  true,
			expectKeyExist: false,
		},
		{
			// Behavior change (D2', scalar-value gap): a bare comparable
			// scalar value is no longer flagged invalid -- it compares
			// like a one-element set through the shared primitive.
			name:           "bare scalar value compares instead of erroring",
			key:            "a",
			value:          12345,
			expectInvalid:  false,
			expectKeyExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalidType, keyExists := keyExistsInArray(tt.key, tt.value, log)
			assert.Equal(t, tt.expectInvalid, invalidType)
			assert.Equal(t, tt.expectKeyExist, keyExists)
		})
	}
}

func TestSetExistsInArray(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name           string
		key            []interface{}
		value          interface{}
		notIn          bool
		expectInvalid  bool
		expectKeyExist bool
	}{
		{
			name:           "all keys in slice - In check",
			key:            []interface{}{"a", "b"},
			value:          []interface{}{"a", "b", "c"},
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "not all keys in slice - In check",
			key:            []interface{}{"a", "d"},
			value:          []interface{}{"a", "b", "c"},
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: false,
		},
		{
			name:           "any key not in slice - NotIn check",
			key:            []interface{}{"a", "d"},
			value:          []interface{}{"a", "b", "c"},
			notIn:          true,
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "all keys in slice - NotIn check",
			key:            []interface{}{"a", "b"},
			value:          []interface{}{"a", "b", "c"},
			notIn:          true,
			expectInvalid:  false,
			expectKeyExist: false,
		},
		{
			name:           "single key matches string value",
			key:            []interface{}{"test"},
			value:          "test",
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			name:           "keys in JSON array",
			key:            []interface{}{"a", "b"},
			value:          `["a", "b", "c"]`,
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			// Behavior change (D2', ruling B): the string-value grammar is
			// harmonized with the Any/All operators -- a non-JSON string is
			// treated as a single-element value instead of an unmarshal
			// error, so this now compares cleanly instead of bailing out.
			name:           "non-JSON string value falls back to a single element",
			key:            []interface{}{"a"},
			value:          `not-json`,
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: false,
		},
		{
			// Behavior change (D2'): a non-string element no longer bails
			// the whole comparison out; it is coerced like any other
			// scalar, so the string element it sits next to still compares.
			name:           "non-string in value slice no longer bails out",
			key:            []interface{}{"a"},
			value:          []interface{}{"a", 123},
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: true,
		},
		{
			// Behavior change (D2', scalar-value gap): same as
			// keyExistsInArray's equivalent case above.
			name:           "bare scalar value compares instead of erroring",
			key:            []interface{}{"a"},
			value:          12345,
			notIn:          false,
			expectInvalid:  false,
			expectKeyExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalidType, keyExists := setExistsInArray(toElements(tt.key), tt.value, log, tt.notIn)
			assert.Equal(t, tt.expectInvalid, invalidType)
			assert.Equal(t, tt.expectKeyExist, keyExists)
		})
	}
}

func TestIsIn(t *testing.T) {
	tests := []struct {
		name     string
		key      []string
		value    []string
		expected bool
	}{
		{
			name:     "all keys in value",
			key:      []string{"a", "b"},
			value:    []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "not all keys in value",
			key:      []string{"a", "d"},
			value:    []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "empty key",
			key:      []string{},
			value:    []string{"a", "b"},
			expected: true,
		},
		{
			name:     "empty value",
			key:      []string{"a"},
			value:    []string{},
			expected: false,
		},
		{
			name:     "both empty",
			key:      []string{},
			value:    []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIn(elementsFromStrings(tt.key), elementsFromStrings(tt.value))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNotIn(t *testing.T) {
	tests := []struct {
		name     string
		key      []string
		value    []string
		expected bool
	}{
		{
			name:     "any key not in value",
			key:      []string{"a", "d"},
			value:    []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "all keys in value",
			key:      []string{"a", "b"},
			value:    []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "empty key - none not in",
			key:      []string{},
			value:    []string{"a", "b"},
			expected: false,
		},
		{
			name:     "key not in empty value",
			key:      []string{"a"},
			value:    []string{},
			expected: true,
		},
		{
			name:     "both empty",
			key:      []string{},
			value:    []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotIn(elementsFromStrings(tt.key), elementsFromStrings(tt.value))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInHandler_UnreachableMethods(t *testing.T) {
	handler := InHandler{}

	// These methods should always return false as per the implementation
	assert.False(t, handler.validateValueWithBoolPattern(true, nil))
	assert.False(t, handler.validateValueWithIntPattern(0, nil))
	assert.False(t, handler.validateValueWithFloatPattern(0.0, nil))
	assert.False(t, handler.validateValueWithMapPattern(nil, nil))
	assert.False(t, handler.validateValueWithSlicePattern(nil, nil))
}
