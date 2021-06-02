package jmespath

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func TestJMESPathFunctions_CompareEqualStrings(t *testing.T) {
	jp, err := New("compare('a', 'a')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(int)
	assert.Assert(t, ok)
	assert.Equal(t, equal, 0)
}

func TestJMESPathFunctions_CompareDifferentStrings(t *testing.T) {
	jp, err := New("compare('a', 'b')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(int)
	assert.Assert(t, ok)
	assert.Equal(t, equal, -1)
}

func TestJMESPathFunctions_Contains(t *testing.T) {
	jp, err := New("contains('string', 'str')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	contains, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Assert(t, contains)
}

func TestJMESPathFunctions_EqualFold(t *testing.T) {
	jp, err := New("equal_fold('Go', 'go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Assert(t, equal)
}

func TestJMESPathFunctions_Replace(t *testing.T) {
	// Can't use integer literals due to
	// https://github.com/jmespath/go-jmespath/issues/27
	//
	// TODO: fix this in https://github.com/kyverno/go-jmespath
	//

	jp, err := New("replace('Lorem ipsum dolor sit amet', 'ipsum', 'muspi', `-1`)")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	replaced, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, replaced, "Lorem muspi dolor sit amet")
}

func TestJMESPathFunctions_ReplaceAll(t *testing.T) {
	jp, err := New("replace_all('Lorem ipsum dolor sit amet', 'ipsum', 'muspi')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	replaced, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, replaced, "Lorem muspi dolor sit amet")
}

func TestJMESPathFunctions_ToUpper(t *testing.T) {
	jp, err := New("to_upper('abc')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	upper, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, upper, "ABC")
}

func TestJMESPathFunctions_ToLower(t *testing.T) {
	jp, err := New("to_lower('AbC')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	lower, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, lower, "abc")
}

func TestJMESPathFunctions_Trim(t *testing.T) {
	jp, err := New("trim('¡¡¡Hello, Gophers!!!', '!¡')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	trim, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, trim, "Hello, Gophers")
}

func TestJMESPathFunctions_Split(t *testing.T) {
	jp, err := New("split('Hello, Gophers', ', ')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.([]string)
	assert.Assert(t, ok)
	assert.Equal(t, split[0], "Hello")
	assert.Equal(t, split[1], "Gophers")
}

func TestJMESPathFunctions_HasPrefix(t *testing.T) {
	jp, err := New("starts_with('Gophers', 'Go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Equal(t, split, true)
}

func TestJMESPathFunctions_HasSuffix(t *testing.T) {
	jp, err := New("ends_with('Amigo', 'go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Equal(t, split, true)
}

func Test_regexMatch(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = "hgf'b1a2r'b12g"

	query, err := New("regex_match('12.*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_regexMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := New("regex_match('12.*', abs(foo))")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_regexReplaceAll(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "ns_first"
		},
		"spec": {
			"namespace": "ns_first",
			"name": "temp_other",
			"field" : "Hello world, helworldlo"
		}
	}
	`)
	expected := "Glo world, Gworldlo"

	var resource interface{}
	err := json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	query, err := New(`regex_replace_all('([Hh]e|G)l', spec.field, '${2}G')`)
	assert.NilError(t, err)

	res, err := query.Search(resource)
	assert.NilError(t, err)

	result, ok := res.(string)
	assert.Assert(t, ok)
	assert.Equal(t, string(result), expected)
}

func Test_regexReplaceAllLiteral(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "ns_first"
		},
		"spec": {
			"namespace": "ns_first",
			"name": "temp_other",
			"field" : "Hello world, helworldlo"
		}
	}
	`)
	expected := "Glo world, Gworldlo"

	var resource interface{}
	err := json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)

	query, err := New(`regex_replace_all_literal('[Hh]el?', spec.field, 'G')`)
	assert.NilError(t, err)

	res, err := query.Search(resource)
	assert.NilError(t, err)

	result, ok := res.(string)
	assert.Assert(t, ok)

	assert.Equal(t, string(result), expected)
}

func Test_labelMatch(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"labels": {
				"app": "test-app",
				"controller-name": "test-controller"
			}
		}
	}
	`)

	testCases := []struct {
		resource       []byte
		test           string
		expectedResult bool
	}{
		{
			resource:       resourceRaw,
			test:           `{ "app": "test-app" }`,
			expectedResult: true,
		},
		{
			resource:       resourceRaw,
			test:           `{ "app": "test-app", "controller-name": "test-controller" }`,
			expectedResult: true,
		},
		{
			resource:       resourceRaw,
			test:           `{ "app": "test-app2" }`,
			expectedResult: false,
		},
		{
			resource:       resourceRaw,
			test:           `{ "app.kubernetes.io/name": "test-app" }`,
			expectedResult: false,
		},
	}

	for _, testCase := range testCases {
		var resource interface{}
		err := json.Unmarshal(testCase.resource, &resource)
		assert.NilError(t, err)

		query, err := New("label_match(`" + testCase.test + "`, metadata.labels)")
		assert.NilError(t, err)

		res, err := query.Search(resource)
		assert.NilError(t, err)

		result, ok := res.(bool)
		assert.Assert(t, ok)

		assert.Equal(t, result, testCase.expectedResult)
	}

}
