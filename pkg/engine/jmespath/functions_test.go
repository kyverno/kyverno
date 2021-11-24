package jmespath

import (
	"encoding/json"
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func Test_Compare(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult int
	}{
		{
			jmesPath:       "compare('a', 'a')",
			expectedResult: 0,
		},
		{
			jmesPath:       "compare('a', 'b')",
			expectedResult: -1,
		},
		{
			jmesPath:       "compare('b', 'a')",
			expectedResult: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := New(tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			res, ok := result.(int)
			assert.Assert(t, ok)
			assert.Equal(t, res, tc.expectedResult)
		})
	}
}

func Test_EqualFold(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult bool
	}{
		{
			jmesPath:       "equal_fold('Go', 'go')",
			expectedResult: true,
		},
		{
			jmesPath:       "equal_fold('a', 'b')",
			expectedResult: false,
		},
		{
			jmesPath:       "equal_fold('1', 'b')",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := New(tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			res, ok := result.(bool)
			assert.Assert(t, ok)
			assert.Equal(t, res, tc.expectedResult)
		})
	}
}

func Test_Replace(t *testing.T) {
	// Can't use integer literals due to
	// https://github.com/jmespath/go-jmespath/issues/27
	//
	// TODO: fix this in https://github.com/kyverno/go-jmespath

	testCases := []struct {
		jmesPath       string
		expectedResult string
	}{
		{
			jmesPath:       "replace('Lorem ipsum dolor sit amet', 'ipsum', 'muspi', `-1`)",
			expectedResult: "Lorem muspi dolor sit amet",
		},
		{
			jmesPath:       "replace('Lorem ipsum ipsum ipsum dolor sit amet', 'ipsum', 'muspi', `-1`)",
			expectedResult: "Lorem muspi muspi muspi dolor sit amet",
		},
		{
			jmesPath:       "replace('Lorem ipsum ipsum ipsum dolor sit amet', 'ipsum', 'muspi', `1`)",
			expectedResult: "Lorem muspi ipsum ipsum dolor sit amet",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := New(tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			res, ok := result.(string)
			assert.Assert(t, ok)
			assert.Equal(t, res, tc.expectedResult)
		})
	}
}

func Test_ReplaceAll(t *testing.T) {
	jp, err := New("replace_all('Lorem ipsum dolor sit amet', 'ipsum', 'muspi')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	replaced, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, replaced, "Lorem muspi dolor sit amet")
}

func Test_ToUpper(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult string
	}{
		{
			jmesPath:       "to_upper('abc')",
			expectedResult: "ABC",
		},
		{
			jmesPath:       "to_upper('123')",
			expectedResult: "123",
		},
		{
			jmesPath:       "to_upper('a#%&123Bc')",
			expectedResult: "A#%&123BC",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := New(tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			res, ok := result.(string)
			assert.Assert(t, ok)
			assert.Equal(t, res, tc.expectedResult)
		})
	}
}

func Test_ToLower(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult string
	}{
		{
			jmesPath:       "to_lower('ABC')",
			expectedResult: "abc",
		},
		{
			jmesPath:       "to_lower('123')",
			expectedResult: "123",
		},
		{
			jmesPath:       "to_lower('a#%&123Bc')",
			expectedResult: "a#%&123bc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := New(tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			res, ok := result.(string)
			assert.Assert(t, ok)
			assert.Equal(t, res, tc.expectedResult)
		})
	}
}

func Test_Trim(t *testing.T) {
	jp, err := New("trim('¡¡¡Hello, Gophers!!!', '!¡')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	trim, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, trim, "Hello, Gophers")
}

func Test_Split(t *testing.T) {
	jp, err := New("split('Hello, Gophers', ', ')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.([]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, split[0], "Hello")
	assert.Equal(t, split[1], "Gophers")
}

func Test_HasPrefix(t *testing.T) {
	jp, err := New("starts_with('Gophers', 'Go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Equal(t, split, true)
}

func Test_HasSuffix(t *testing.T) {
	jp, err := New("ends_with('Amigo', 'go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Equal(t, split, true)
}

func Test_RegexMatch(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = "hgf'b1a2r'b12g"

	query, err := New("regex_match('12.*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_RegexMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := New("regex_match('12.*', abs(foo))")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_RegexReplaceAll(t *testing.T) {
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

func Test_RegexReplaceAllLiteral(t *testing.T) {
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

func Test_LabelMatch(t *testing.T) {
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

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			var resource interface{}
			err := json.Unmarshal(tc.resource, &resource)
			assert.NilError(t, err)

			query, err := New("label_match(`" + tc.test + "`, metadata.labels)")
			assert.NilError(t, err)

			res, err := query.Search(resource)
			assert.NilError(t, err)

			result, ok := res.(bool)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}

}

func Test_Add(t *testing.T) {
	jp, err := New("add(`12`, `13`)")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(float64)
	assert.Assert(t, ok)
	assert.Equal(t, equal, 25.0)
}

func Test_Subtract(t *testing.T) {
	jp, err := New("subtract(`12`, `7`)")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(float64)
	assert.Assert(t, ok)
	assert.Equal(t, equal, 5.0)
}

func Test_Multiply(t *testing.T) {
	jp, err := New("multiply(`3`, `2.5`)")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(float64)
	assert.Assert(t, ok)
	assert.Equal(t, equal, 7.5)
}

func Test_Divide(t *testing.T) {
	jp, err := New("divide(`12`, `1.5`)")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(float64)
	assert.Assert(t, ok)
	assert.Equal(t, equal, 8.0)
}

func Test_Modulo(t *testing.T) {
	jp, err := New("modulo(`12`, `7`)")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	equal, ok := result.(int64)
	assert.Assert(t, ok)
	assert.Equal(t, equal, int64(5))
}

func Test_Base64Decode(t *testing.T) {
	jp, err := New("base64_decode('SGVsbG8sIHdvcmxkIQ==')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	str, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, str, "Hello, world!")
}

func Test_Base64Encode(t *testing.T) {
	jp, err := New("base64_encode('Hello, world!')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	str, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, str, "SGVsbG8sIHdvcmxkIQ==")
}

func Test_Base64Decode_Secret(t *testing.T) {
	resourceRaw := []byte(`
	{
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": "example",
        "namespace": "default"
    },
    "type": "Opaque",
    "data": {
        "example1": "SGVsbG8sIHdvcmxkIQ==",
        "example2": "Rm9vQmFy"
    }
	}
	`)

	var resource interface{}
	err := json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	query, err := New(`base64_decode(data.example1)`)
	assert.NilError(t, err)

	res, err := query.Search(resource)
	assert.NilError(t, err)

	result, ok := res.(string)
	assert.Assert(t, ok)
	assert.Equal(t, string(result), "Hello, world!")
}

func Test_TimeSince(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult string
	}{
		{
			test:           "time_since('', '2021-01-02T15:04:05-07:00', '2021-01-10T03:14:05-07:00')",
			expectedResult: "180h10m0s",
		},
		{
			test:           "time_since('Mon Jan _2 15:04:05 MST 2006', 'Mon Jan 02 15:04:05 MST 2021', 'Mon Jan 10 03:14:16 MST 2021')",
			expectedResult: "180h10m11s",
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := New(tc.test)
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.(string)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}
