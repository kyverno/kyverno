package jmespath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgumentTypeErrors(t *testing.T) {
	data := map[string]any{
		"null":   nil,
		"string": "abc",
		"number": 1.0,
		"bool":   true,
		"array":  []any{"a"},
		"object": map[string]any{"a": "b"},
	}
	testCases := []struct {
		name  string
		query string
		want  string
	}{{
		name:  "null argument",
		query: "base64_decode(null)",
		want:  "JMESPath function 'base64_decode': argument #1 is not of type string (got null)",
	}, {
		name:  "number argument",
		query: "sha256(number)",
		want:  "JMESPath function 'sha256': argument #1 is not of type string (got number)",
	}, {
		name:  "boolean argument",
		query: "md5(bool)",
		want:  "JMESPath function 'md5': argument #1 is not of type string (got bool)",
	}, {
		name:  "array argument",
		query: "sha1(array)",
		want:  "JMESPath function 'sha1': argument #1 is not of type string (got array)",
	}, {
		name:  "object argument",
		query: "is_external_url(object)",
		want:  "JMESPath function 'is_external_url': argument #1 is not of type string (got object)",
	}, {
		name:  "second argument",
		query: "time_add(string, null)",
		want:  "JMESPath function 'time_add': argument #2 is not of type string (got null)",
	}, {
		name:  "union of accepted types",
		query: "items(string, string, string)",
		want:  "JMESPath function 'items': argument #1 is not of type object or array (got string)",
	}, {
		name:  "wildcard pattern",
		query: "pattern_match(array, string)",
		want:  "JMESPath function 'pattern_match': argument #1 is not of type string (got array)",
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := jmespathInterface.Query(tc.query)
			assert.NoError(t, err)
			_, err = jp.Search(data)
			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestArgumentTypesAccepted(t *testing.T) {
	data := map[string]any{
		"string":  "abc",
		"number":  1.0,
		"numbers": []any{1.0, 2.0},
		"object":  map[string]any{"a": "b"},
	}
	testCases := []struct {
		name  string
		query string
		want  any
	}{{
		name:  "string",
		query: "to_upper(string)",
		want:  "ABC",
	}, {
		name:  "number",
		query: "round(number, `0`)",
		want:  1.0,
	}, {
		name:  "array",
		query: "sum(numbers)",
		want:  3.0,
	}, {
		name:  "object accepted by a union spec",
		query: "lookup(object, 'a')",
		want:  "b",
	}, {
		name:  "array accepted by a union spec",
		query: "lookup(numbers, `1`)",
		want:  2.0,
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := jmespathInterface.Query(tc.query)
			assert.NoError(t, err)
			got, err := jp.Search(data)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTypeName(t *testing.T) {
	assert.Equal(t, "null", typeName(nil))
	assert.Equal(t, "string", typeName("a"))
	assert.Equal(t, "number", typeName(1.0))
	assert.Equal(t, "bool", typeName(true))
	assert.Equal(t, "array", typeName([]any{}))
	assert.Equal(t, "object", typeName(map[string]any{}))
	assert.Equal(t, "int", typeName(1))
}
