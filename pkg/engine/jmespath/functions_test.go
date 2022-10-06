package jmespath

import (
	"encoding/json"
	"fmt"
	"runtime"
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

func Test_ParseJsonSerde(t *testing.T) {
	testCases := []string{
		`{"a":"b"}`,
		`true`,
		`[1,2,3,{"a":"b"}]`,
		`null`,
		`[]`,
		`{}`,
		`0`,
		`1.2`,
		`[1.2,true,{"a":{"a":"b"}}]`,
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			jp, err := New(fmt.Sprintf(`to_string(parse_json('%s'))`, tc))
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			assert.Equal(t, result, tc)
		})
	}
}

func Test_ParseJsonComplex(t *testing.T) {
	testCases := []struct {
		input          string
		expectedResult interface{}
	}{
		{
			input:          `parse_json('{"a": "b"}').a`,
			expectedResult: "b",
		},
		{
			input:          `parse_json('{"a": [1, 2, 3, 4]}').a[0]`,
			expectedResult: 1.0,
		},
		{
			input:          `parse_json('[1, 2, {"a": {"b": {"c": [1, 2]}}}]')[2].a.b.c[1]`,
			expectedResult: 2.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			jp, err := New(tc.input)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_ParseYAML(t *testing.T) {
	testCases := []struct {
		input  string
		output interface{}
	}{
		{
			input: `a: b`,
			output: map[string]interface{}{
				"a": "b",
			},
		},
		{
			input: `
- 1
- 2
- 3
- a: b`,
			output: []interface{}{
				1.0,
				2.0,
				3.0,
				map[string]interface{}{
					"a": "b",
				},
			},
		},
		{
			input: `
spec:
  test: 1
  test2:
    - 2
    - 3
`,
			output: map[string]interface{}{
				"spec": map[string]interface{}{
					"test":  1.0,
					"test2": []interface{}{2.0, 3.0},
				},
			},
		},
		{
			input: `
bar: >
  this is not a normal string it
  spans more than
  one line
  see?`,
			output: map[string]interface{}{
				"bar": "this is not a normal string it spans more than one line see?",
			},
		},
		{
			input: `
---
foo: ~
bar: null
`,
			output: map[string]interface{}{
				"bar": nil,
				"foo": nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			jp, err := New(fmt.Sprintf(`parse_yaml('%s')`, tc.input))
			assert.NilError(t, err)
			result, err := jp.Search("")
			assert.NilError(t, err)
			assert.DeepEqual(t, result, tc.output)
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

func Test_PatternMatch(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = "prefix-foo"

	query, err := New("pattern_match('prefix-*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_PatternMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := New("pattern_match('12*', abs(foo))")
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
	testCases := []struct {
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		{
			test: "add('12', '13s')",
			err:  true,
		},
		{
			test: "add('12Ki', '13s')",
			err:  true,
		},
		{
			test: "add('12s', '13')",
			err:  true,
		},
		{
			test: "add('12s', '13Ki')",
			err:  true,
		},
		{
			test:           "add(`12`, `13`)",
			expectedResult: 25.0,
			retFloat:       true,
		},
		{
			test:           "add(`12`, '13s')",
			expectedResult: `25s`,
		},
		{
			test:           "add('12s', '13s')",
			expectedResult: `25s`,
		},
		{
			test:           "add(`12`, '-13s')",
			expectedResult: `-1s`,
		},
		{
			test:           "add(`12`, '13Ki')",
			expectedResult: `13324`,
		},
		{
			test:           "add('12Ki', '13Ki')",
			expectedResult: `25Ki`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			jp, err := New(tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Subtract(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		{
			test: "subtract('12', '13s')",
			err:  true,
		},
		{
			test: "subtract('12Ki', '13s')",
			err:  true,
		},
		{
			test: "subtract('12s', '13')",
			err:  true,
		},
		{
			test: "subtract('12s', '13Ki')",
			err:  true,
		},
		{
			test:           "subtract(`12`, `13`)",
			expectedResult: -1.0,
			retFloat:       true,
		},
		{
			test:           "subtract(`12`, '13s')",
			expectedResult: `-1s`,
		},
		{
			test:           "subtract('12s', '13s')",
			expectedResult: `-1s`,
		},
		{
			test:           "subtract(`12`, '-13s')",
			expectedResult: `25s`,
		},
		{
			test:           "subtract(`12`, '13Ki')",
			expectedResult: `-13300`,
		},
		{
			test:           "subtract('12Ki', '13Ki')",
			expectedResult: `-1Ki`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			jp, err := New(tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Multiply(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		{
			test: "multiply('12', '13s')",
			err:  true,
		},
		{
			test: "multiply('12Ki', '13s')",
			err:  true,
		},
		{
			test: "multiply('12s', '13')",
			err:  true,
		},
		{
			test: "multiply('12s', '13Ki')",
			err:  true,
		},
		{
			test:           "multiply(`12`, `13`)",
			expectedResult: 156.0,
			retFloat:       true,
		},
		{
			test:           "multiply(`12`, '13s')",
			expectedResult: `2m36s`,
		},
		{
			test:           "multiply('12s', '13s')",
			expectedResult: `2m36s`,
		},
		{
			test:           "multiply(`12`, '-13s')",
			expectedResult: `-2m36s`,
		},
		{
			test:           "multiply(`12`, '13Ki')",
			expectedResult: `156Ki`,
		},
		{
			test:           "multiply('12Ki', '13Ki')",
			expectedResult: `156Mi`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			jp, err := New(tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Divide(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		{
			test: "divide('12', '13s')",
			err:  true,
		},
		{
			test: "divide('12Ki', '13s')",
			err:  true,
		},
		{
			test: "divide('12s', '13')",
			err:  true,
		},
		{
			test: "divide('12s', '13Ki')",
			err:  true,
		},
		{
			test: "divide('12s', `0`)",
			err:  true,
		},
		{
			test: "divide('12s', '0s')",
			err:  true,
		},
		{
			test: "divide(`12`, '0s')",
			err:  true,
		},
		{
			test: "divide('12M', '0Mi')",
			err:  true,
		},
		{
			test: "divide('12K', `0`)",
			err:  true,
		},
		{
			test: "divide('12K', '0m')",
			err:  true,
		},
		{
			test: "divide('12Ki', '0G')",
			err:  true,
		},
		{
			test: "divide('12Mi', '0Gi')",
			err:  true,
		},
		{
			test: "divide('12Mi', `0`)",
			err:  true,
		},
		{
			test: "divide(`12`, '0Gi')",
			err:  true,
		},
		{
			test: "divide(`12`, '0K')",
			err:  true,
		},
		{
			test: "divide(`12`, `0`)",
			err:  true,
		},
		{
			test:           "divide(`25`, `2`)",
			expectedResult: 12.5,
			retFloat:       true,
		},
		{
			test:           "divide(`12`, '2s')",
			expectedResult: `6s`,
		},
		{
			test:           "divide('25m0s', '2s')",
			expectedResult: 750.0,
			retFloat:       true,
		},
		{
			test:           "divide(`360`, '-2s')",
			expectedResult: `-3m0s`,
		},
		{
			test:           "divide(`13312`, '1Ki')",
			expectedResult: `13`,
		},
		{
			test:           "divide('26Gi', '13Ki')",
			expectedResult: 2097152.0,
			retFloat:       true,
		},
		{
			test:           "divide('500m', `2`)",
			expectedResult: `250m`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			jp, err := New(tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Modulo(t *testing.T) {
	testCases := []struct {
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		{
			test: "modulo('12', '13s')",
			err:  true,
		},
		{
			test: "modulo('12Ki', '13s')",
			err:  true,
		},
		{
			test: "modulo('12s', '13')",
			err:  true,
		},
		{
			test: "modulo('12s', '13Ki')",
			err:  true,
		},
		{
			test: "modulo('12s', `0`)",
			err:  true,
		},
		{
			test: "modulo('12s', '0s')",
			err:  true,
		},
		{
			test: "modulo(`12`, '0s')",
			err:  true,
		},
		{
			test: "modulo('12M', '0Mi')",
			err:  true,
		},
		{
			test: "modulo('12K', `0`)",
			err:  true,
		},
		{
			test: "modulo('12K', '0m')",
			err:  true,
		},
		{
			test: "modulo('12Ki', '0G')",
			err:  true,
		},
		{
			test: "modulo('12Mi', '0Gi')",
			err:  true,
		},
		{
			test: "modulo('12Mi', `0`)",
			err:  true,
		},
		{
			test: "modulo(`12`, '0Gi')",
			err:  true,
		},
		{
			test: "modulo(`12`, '0K')",
			err:  true,
		},
		{
			test: "modulo(`12`, `0`)",
			err:  true,
		},
		{
			test:           "modulo(`25`, `2`)",
			expectedResult: 1.0,
			retFloat:       true,
		},
		{
			test:           "modulo(`13`, '2s')",
			expectedResult: `1s`,
		},
		{
			test:           "modulo('25m13s', '32s')",
			expectedResult: `9s`,
		},
		{
			test:           "modulo(`371`, '-13s')",
			expectedResult: `7s`,
		},
		{
			test:           "modulo(`13312`, '513')",
			expectedResult: `487`,
		},
		{
			test:           "modulo('26Gi', '12Ki')",
			expectedResult: `8Ki`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			jp, err := New(tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
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

func Test_PathCanonicalize(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult string
	}{
		{
			jmesPath:       "path_canonicalize('///')",
			expectedResult: "/",
		},
		{
			jmesPath:       "path_canonicalize('///var/run/containerd/containerd.sock')",
			expectedResult: "/var/run/containerd/containerd.sock",
		},
		{
			jmesPath:       "path_canonicalize('/var/run///containerd/containerd.sock')",
			expectedResult: "/var/run/containerd/containerd.sock",
		},
		{
			jmesPath:       "path_canonicalize('/var/run///containerd////')",
			expectedResult: "/var/run/containerd",
		},
		{
			jmesPath:       "path_canonicalize('/run///')",
			expectedResult: "/run",
		},
		{
			jmesPath:       "path_canonicalize('/run/../etc')",
			expectedResult: "/etc",
		},
		{
			jmesPath:       "path_canonicalize('///etc*')",
			expectedResult: "/etc*",
		},
		{
			jmesPath:       "path_canonicalize('/../../')",
			expectedResult: "/",
		},
	}

	testCasesForWindows := []struct {
		jmesPath       string
		expectedResult string
	}{
		{
			jmesPath:       "path_canonicalize('C:\\Windows\\\\..')",
			expectedResult: "C:\\",
		},
		{
			jmesPath:       "path_canonicalize('C:\\Windows\\\\...')",
			expectedResult: "C:\\Windows\\...",
		},
		{
			jmesPath:       "path_canonicalize('C:\\Users\\USERNAME\\\\\\Downloads')",
			expectedResult: "C:\\Users\\USERNAME\\Downloads",
		},
		{
			jmesPath:       "path_canonicalize('C:\\Users\\\\USERNAME\\..\\Downloads')",
			expectedResult: "C:\\Users\\Downloads",
		},
	}

	if runtime.GOOS == "windows" {
		testCases = testCasesForWindows
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

func Test_Truncate(t *testing.T) {
	// Can't use integer literals due to
	// https://github.com/jmespath/go-jmespath/issues/27
	//
	// TODO: fix this in https://github.com/kyverno/go-jmespath

	testCases := []struct {
		jmesPath       string
		expectedResult string
	}{
		{
			jmesPath:       "truncate('Lorem ipsum dolor sit amet', `5`)",
			expectedResult: "Lorem",
		},
		{
			jmesPath:       "truncate('Lorem ipsum ipsum ipsum dolor sit amet', `11`)",
			expectedResult: "Lorem ipsum",
		},
		{
			jmesPath:       "truncate('Lorem ipsum ipsum ipsum dolor sit amet', `40`)",
			expectedResult: "Lorem ipsum ipsum ipsum dolor sit amet",
		},
		{
			jmesPath:       "truncate('Lorem ipsum', `2.6`)",
			expectedResult: "Lo",
		},
		{
			jmesPath:       "truncate('Lorem ipsum', `0`)",
			expectedResult: "",
		},
		{
			jmesPath:       "truncate('Lorem ipsum', `-1`)",
			expectedResult: "",
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

func Test_SemverCompare(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult bool
	}{
		{
			jmesPath:       "semver_compare('4.1.3','>=4.1.x')",
			expectedResult: true,
		},
		{
			jmesPath:       "semver_compare('4.1.3','!4.x.x')",
			expectedResult: false,
		},
		{
			jmesPath:       "semver_compare('1.8.6','>1.0.0 <2.0.0')", // >1.0.0 AND <2.0.0
			expectedResult: true,
		},
		{
			jmesPath:       "semver_compare('2.1.5','<2.0.0 || >=3.0.0')", // <2.0.0 OR >=3.0.0
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

func Test_Items(t *testing.T) {

	testCases := []struct {
		object         string
		keyName        string
		valName        string
		expectedResult string
	}{
		{
			object:         `{ "key1": "value1" }`,
			keyName:        `"key"`,
			valName:        `"value"`,
			expectedResult: `[{ "key": "key1", "value": "value1" }]`,
		},
		{
			object:         `{ "key1": "value1", "key2": "value2" }`,
			keyName:        `"key"`,
			valName:        `"value"`,
			expectedResult: `[{ "key": "key1", "value": "value1" }, { "key": "key2", "value": "value2" }]`,
		},
		{
			object:         `{ "key1": "value1", "key2": "value2" }`,
			keyName:        `"myKey"`,
			valName:        `"myValue"`,
			expectedResult: `[{ "myKey": "key1", "myValue": "value1" }, { "myKey": "key2", "myValue": "value2" }]`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {

			query, err := New("items(`" + tc.object + "`,`" + tc.keyName + "`,`" + tc.valName + "`)")
			assert.NilError(t, err)

			res, err := query.Search("")
			assert.NilError(t, err)

			result, ok := res.([]map[string]interface{})
			assert.Assert(t, ok)

			var resource []map[string]interface{}
			err = json.Unmarshal([]byte(tc.expectedResult), &resource)
			assert.NilError(t, err)

			assert.DeepEqual(t, result, resource)
		})
	}

}

func Test_ObjectFromLists(t *testing.T) {

	testCases := []struct {
		keys           string
		values         string
		expectedResult map[string]interface{}
	}{
		{
			keys:   `["key1", "key2"]`,
			values: `["1", "2"]`,
			expectedResult: map[string]interface{}{
				"key1": "1",
				"key2": "2",
			},
		},
		{
			keys:   `["key1", "key2"]`,
			values: `[1, "2"]`,
			expectedResult: map[string]interface{}{
				"key1": 1.0,
				"key2": "2",
			},
		},
		{
			keys:   `["key1", "key2"]`,
			values: `[1]`,
			expectedResult: map[string]interface{}{
				"key1": 1.0,
				"key2": nil,
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := New("object_from_lists(`" + tc.keys + "`,`" + tc.values + "`)")
			assert.NilError(t, err)
			res, err := query.Search("")
			assert.NilError(t, err)
			result, ok := res.(map[string]interface{})
			assert.Assert(t, ok)
			assert.DeepEqual(t, result, tc.expectedResult)
		})
	}

}

func Test_x509Decode(t *testing.T) {
	certs := []string{
		"LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUM3VENDQWRXZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFZTVJZd0ZBWURWUVFEREEwcUxtdDUKZG1WeWJtOHVjM1pqTUI0WERUSXlNREV4TVRFek1qWTBNMW9YRFRJek1ERXhNVEUwTWpZME0xb3dHREVXTUJRRwpBMVVFQXd3TktpNXJlWFpsY201dkxuTjJZekNDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DCmdnRUJBTXNBejg1K3lpbm8rTW1kS3NWdEh3Tmkzb0FWanVtelhIaUxmVUpLN3hpNUtVOEI3Z29QSEYvVkNlL1YKN1kyYzRhZnlmZ1kyZVB3NEx4U0RrQ1lOZ1l3cWpTd0dJYmNzcXY1WlJhekJkRHhSMDlyaTZQa25OeUJWR0xpNQpSbFBYSXJHUTNwc051ZjU1cXd4SnhMTzMxcUNadXZrdEtZNVl2dUlSNEpQbUJodVNGWE9ubjBaaVF3OHV4TWNRCjBRQTJseitQeFdDVk5rOXErMzFINURIMW9ZWkRMZlUzbWlqSU9BK0FKR1piQmIrWndCbXBWTDArMlRYTHhFNzQKV293ZEtFVitXVHNLb2pOVGQwVndjdVJLUktSLzZ5blhBQWlzMjF5MVg3VWk5RkpFNm1ESXlsVUQ0MFdYT0tHSgoxbFlZNDFrUm5ZaFZodlhZTjlKdE5ZZFkzSHNDQXdFQUFhTkNNRUF3RGdZRFZSMFBBUUgvQkFRREFnS2tNQThHCkExVWRFd0VCL3dRRk1BTUJBZjh3SFFZRFZSME9CQllFRk9ubEFTVkQ5ZnUzVEFqcHRsVy9nQVhBNHFsK01BMEcKQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUNJcHlSaUNoeHA5N2NyS2ZRMjRKdDd6OFArQUdwTGYzc1g0ZUw4N0VTYQo3UVJvVkp0WExtYXV0MXBVRW9ZTFFydUttaC8wWUZ0Wkc5V3hWZ1k2aXVLYldudTdiT2VNQi9JcitWL3lyWDNSCitYdlpPc3VYaUpuRWJKaUJXNmxKekxsZG9XNGYvNzFIK2oxV0Q0dEhwcW1kTXhxL3NMcVhmUEl1YzAvbTB5RkMKbitBREJXR0dCOE5uNjZ2eHR2K2NUNnArUklWb3RYUFFXYk1pbFdwNnBkNXdTdUI2OEZxckR3dFlMTkp0UHdGcwo5TVBWa3VhSmRZWjBlV2Qvck1jS0Q5NEhnZjg5Z3ZBMCtxek1WRmYrM0JlbVhza2pRUll5NkNLc3FveUM2alg0Cm5oWWp1bUFQLzdwc2J6SVRzbnBIdGZDRUVVKzJKWndnTTQwNmFpTWNzZ0xiCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K",
		"LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURTakNDQWpLZ0F3SUJBZ0lVV3htajQwbCtURFZKcTk4WHk3YzZMZW8zbnA4d0RRWUpLb1pJaHZjTkFRRUwKQlFBd1BURUxNQWtHQTFVRUJoTUNlSGd4Q2pBSUJnTlZCQWdUQVhneENqQUlCZ05WQkFjVEFYZ3hDakFJQmdOVgpCQW9UQVhneENqQUlCZ05WQkFzVEFYZ3dIaGNOTVRnd01qQXlNVEl6T0RBd1doY05Nak13TWpBeE1USXpPREF3CldqQTlNUXN3Q1FZRFZRUUdFd0o0ZURFS01BZ0dBMVVFQ0JNQmVERUtNQWdHQTFVRUJ4TUJlREVLTUFnR0ExVUUKQ2hNQmVERUtNQWdHQTFVRUN4TUJlRENDQVNJd0RRWUpLb1pJaHZjTkFRRUJCUUFEZ2dFUEFEQ0NBUW9DZ2dFQgpBTkhrcU9tVmYyM0tNWGRhWlUyZUZVeDFoNHdiMDlKSU5CQjh4L0hMN1VFMEtGSmNuT29Wbk5RQjBnUnVrVW9wCmlZQ3pyek1GeUdXV21CL3BBRUtvb2wrWmlJMnVNeTZtY1lCRHRPaTRwT203VTBUUVFNVjZMLzVZZmk2NXhSejMKUlRNZC90WUFvRmk0YUNaYkpBR2p4VTZVV05ZRHpUeThFL2NQNlpubE5iVkhSaUE2L3dIc29XY1h0V1RYWVA1eQpuOWNmN0VXUWkxaE9CTTRCV21PSXlCMWY2TEVnUWlwWldNT01QUEhPM2hzdVNCbjByazdqb3ZTdDVYVGxiZ1JyCnR4cUFKaU5qSlV5a1d6SUYrbExuWkNpb2lwcEd2NXZrZEd2RTgzSm9BQ1h2WlRVd3pBK01MdTQ5Zmt3M2J3ZXEKa2JocmVyOGthY2pmR2x3M2FKTjM3ZUVDQXdFQUFhTkNNRUF3RGdZRFZSMFBBUUgvQkFRREFnRUdNQThHQTFVZApFd0VCL3dRRk1BTUJBZjh3SFFZRFZSME9CQllFRktYY2I1MmJ2Nm9xbkQrRDlmVE5GSFpMOElXeE1BMEdDU3FHClNJYjNEUUVCQ3dVQUE0SUJBUUFEdkt2djN5bTBYQVl3S3hQTExsM0xjNnNKWUhEYlROMGRvbmR1RzdQWGViMWQKaHV1a0oybGZ1ZlVZcDJJR1NBeHVMZWNUWWVlQnlPVnAxZ2FNYjVMc0lHdDJCVkRtbE1Na2lIMjlMVUhzdmJ5aQo4NUNwSm83QTVSSkc2QVdXMlZCQ2lEano1djhKRk02cE1rQlJGZlhIK3B3SWdlNjVDRStNVFNRY2ZiMS9hSUlvClEyMjZQN0UvM3VVR1g0azRwRFhHL083R052eWtGNDB2MURCNXk3RERCVFE0SldpSmZ5R2tUNjlUbWRPR0xGQW0Kand4VWpXeXZFZXk0cUpleC9FR0VtNVJRY012OWl5N3RiYTF3SzdzeWtOR241dURFTEdQR0lJRUFhNXJJSG0xRgpVRk9aWlZvRUxhYXNXUzU1OXd5OG9nMzlFcTIxZERNeW5iOEJuZG4vCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K",
		"-----BEGIN CERTIFICATE-----\nMIIC7TCCAdWgAwIBAgIBADANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDDA0qLmt5\ndmVybm8uc3ZjMB4XDTIyMDExMTEzMjY0M1oXDTIzMDExMTE0MjY0M1owGDEWMBQG\nA1UEAwwNKi5reXZlcm5vLnN2YzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC\nggEBAMsAz85+yino+MmdKsVtHwNi3oAVjumzXHiLfUJK7xi5KU8B7goPHF/VCe/V\n7Y2c4afyfgY2ePw4LxSDkCYNgYwqjSwGIbcsqv5ZRazBdDxR09ri6PknNyBVGLi5\nRlPXIrGQ3psNuf55qwxJxLO31qCZuvktKY5YvuIR4JPmBhuSFXOnn0ZiQw8uxMcQ\n0QA2lz+PxWCVNk9q+31H5DH1oYZDLfU3mijIOA+AJGZbBb+ZwBmpVL0+2TXLxE74\nWowdKEV+WTsKojNTd0VwcuRKRKR/6ynXAAis21y1X7Ui9FJE6mDIylUD40WXOKGJ\n1lYY41kRnYhVhvXYN9JtNYdY3HsCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8G\nA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFOnlASVD9fu3TAjptlW/gAXA4ql+MA0G\nCSqGSIb3DQEBCwUAA4IBAQCIpyRiChxp97crKfQ24Jt7z8P+AGpLf3sX4eL87ESa\n7QRoVJtXLmaut1pUEoYLQruKmh/0YFtZG9WxVgY6iuKbWnu7bOeMB/Ir+V/yrX3R\n+XvZOsuXiJnEbJiBW6lJzLldoW4f/71H+j1WD4tHpqmdMxq/sLqXfPIuc0/m0yFC\nn+ADBWGGB8Nn66vxtv+cT6p+RIVotXPQWbMilWp6pd5wSuB68FqrDwtYLNJtPwFs\n9MPVkuaJdYZ0eWd/rMcKD94Hgf89gvA0+qzMVFf+3BemXskjQRYy6CKsqoyC6jX4\nnhYjumAP/7psbzITsnpHtfCEEU+2JZwgM406aiMcsgLb\n-----END CERTIFICATE-----",
		"-----BEGIN CERTIFICATE-----\nMIIDSjCCAjKgAwIBAgIUWxmj40l+TDVJq98Xy7c6Leo3np8wDQYJKoZIhvcNAQEL\nBQAwPTELMAkGA1UEBhMCeHgxCjAIBgNVBAgTAXgxCjAIBgNVBAcTAXgxCjAIBgNV\nBAoTAXgxCjAIBgNVBAsTAXgwHhcNMTgwMjAyMTIzODAwWhcNMjMwMjAxMTIzODAw\nWjA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UE\nChMBeDEKMAgGA1UECxMBeDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB\nANHkqOmVf23KMXdaZU2eFUx1h4wb09JINBB8x/HL7UE0KFJcnOoVnNQB0gRukUop\niYCzrzMFyGWWmB/pAEKool+ZiI2uMy6mcYBDtOi4pOm7U0TQQMV6L/5Yfi65xRz3\nRTMd/tYAoFi4aCZbJAGjxU6UWNYDzTy8E/cP6ZnlNbVHRiA6/wHsoWcXtWTXYP5y\nn9cf7EWQi1hOBM4BWmOIyB1f6LEgQipZWMOMPPHO3hsuSBn0rk7jovSt5XTlbgRr\ntxqAJiNjJUykWzIF+lLnZCioippGv5vkdGvE83JoACXvZTUwzA+MLu49fkw3bweq\nkbhrer8kacjfGlw3aJN37eECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1Ud\nEwEB/wQFMAMBAf8wHQYDVR0OBBYEFKXcb52bv6oqnD+D9fTNFHZL8IWxMA0GCSqG\nSIb3DQEBCwUAA4IBAQADvKvv3ym0XAYwKxPLLl3Lc6sJYHDbTN0donduG7PXeb1d\nhuukJ2lfufUYp2IGSAxuLecTYeeByOVp1gaMb5LsIGt2BVDmlMMkiH29LUHsvbyi\n85CpJo7A5RJG6AWW2VBCiDjz5v8JFM6pMkBRFfXH+pwIge65CE+MTSQcfb1/aIIo\nQ226P7E/3uUGX4k4pDXG/O7GNvykF40v1DB5y7DDBTQ4JWiJfyGkT69TmdOGLFAm\njwxUjWyvEey4qJex/EGEm5RQcMv9iy7tba1wK7sykNGn5uDELGPGIIEAa5rIHm1F\nUFOZZVoELaasWS559wy8og39Eq21dDMynb8Bndn/\n-----END CERTIFICATE-----",
		`-----BEGIN CERTIFICATE-----
MIIC7TCCAdWgAwIBAgIBADANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDDA0qLmt5
dmVybm8uc3ZjMB4XDTIyMDExMTEzMjY0M1oXDTIzMDExMTE0MjY0M1owGDEWMBQG
A1UEAwwNKi5reXZlcm5vLnN2YzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAMsAz85+yino+MmdKsVtHwNi3oAVjumzXHiLfUJK7xi5KU8B7goPHF/VCe/V
7Y2c4afyfgY2ePw4LxSDkCYNgYwqjSwGIbcsqv5ZRazBdDxR09ri6PknNyBVGLi5
RlPXIrGQ3psNuf55qwxJxLO31qCZuvktKY5YvuIR4JPmBhuSFXOnn0ZiQw8uxMcQ
0QA2lz+PxWCVNk9q+31H5DH1oYZDLfU3mijIOA+AJGZbBb+ZwBmpVL0+2TXLxE74
WowdKEV+WTsKojNTd0VwcuRKRKR/6ynXAAis21y1X7Ui9FJE6mDIylUD40WXOKGJ
1lYY41kRnYhVhvXYN9JtNYdY3HsCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8G
A1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFOnlASVD9fu3TAjptlW/gAXA4ql+MA0G
CSqGSIb3DQEBCwUAA4IBAQCIpyRiChxp97crKfQ24Jt7z8P+AGpLf3sX4eL87ESa
7QRoVJtXLmaut1pUEoYLQruKmh/0YFtZG9WxVgY6iuKbWnu7bOeMB/Ir+V/yrX3R
+XvZOsuXiJnEbJiBW6lJzLldoW4f/71H+j1WD4tHpqmdMxq/sLqXfPIuc0/m0yFC
n+ADBWGGB8Nn66vxtv+cT6p+RIVotXPQWbMilWp6pd5wSuB68FqrDwtYLNJtPwFs
9MPVkuaJdYZ0eWd/rMcKD94Hgf89gvA0+qzMVFf+3BemXskjQRYy6CKsqoyC6jX4
nhYjumAP/7psbzITsnpHtfCEEU+2JZwgM406aiMcsgLb
-----END CERTIFICATE-----
`,
		`-----BEGIN CERTIFICATE-----
MIIDSjCCAjKgAwIBAgIUWxmj40l+TDVJq98Xy7c6Leo3np8wDQYJKoZIhvcNAQEL
BQAwPTELMAkGA1UEBhMCeHgxCjAIBgNVBAgTAXgxCjAIBgNVBAcTAXgxCjAIBgNV
BAoTAXgxCjAIBgNVBAsTAXgwHhcNMTgwMjAyMTIzODAwWhcNMjMwMjAxMTIzODAw
WjA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UE
ChMBeDEKMAgGA1UECxMBeDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
ANHkqOmVf23KMXdaZU2eFUx1h4wb09JINBB8x/HL7UE0KFJcnOoVnNQB0gRukUop
iYCzrzMFyGWWmB/pAEKool+ZiI2uMy6mcYBDtOi4pOm7U0TQQMV6L/5Yfi65xRz3
RTMd/tYAoFi4aCZbJAGjxU6UWNYDzTy8E/cP6ZnlNbVHRiA6/wHsoWcXtWTXYP5y
n9cf7EWQi1hOBM4BWmOIyB1f6LEgQipZWMOMPPHO3hsuSBn0rk7jovSt5XTlbgRr
txqAJiNjJUykWzIF+lLnZCioippGv5vkdGvE83JoACXvZTUwzA+MLu49fkw3bweq
kbhrer8kacjfGlw3aJN37eECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1Ud
EwEB/wQFMAMBAf8wHQYDVR0OBBYEFKXcb52bv6oqnD+D9fTNFHZL8IWxMA0GCSqG
SIb3DQEBCwUAA4IBAQADvKvv3ym0XAYwKxPLLl3Lc6sJYHDbTN0donduG7PXeb1d
huukJ2lfufUYp2IGSAxuLecTYeeByOVp1gaMb5LsIGt2BVDmlMMkiH29LUHsvbyi
85CpJo7A5RJG6AWW2VBCiDjz5v8JFM6pMkBRFfXH+pwIge65CE+MTSQcfb1/aIIo
Q226P7E/3uUGX4k4pDXG/O7GNvykF40v1DB5y7DDBTQ4JWiJfyGkT69TmdOGLFAm
jwxUjWyvEey4qJex/EGEm5RQcMv9iy7tba1wK7sykNGn5uDELGPGIIEAa5rIHm1F
UFOZZVoELaasWS559wy8og39Eq21dDMynb8Bndn/
-----END CERTIFICATE-----
`,
	}
	resList := []string{
		`{"Raw":"MIIC7TCCAdWgAwIBAgIBADANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDDA0qLmt5dmVybm8uc3ZjMB4XDTIyMDExMTEzMjY0M1oXDTIzMDExMTE0MjY0M1owGDEWMBQGA1UEAwwNKi5reXZlcm5vLnN2YzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMsAz85+yino+MmdKsVtHwNi3oAVjumzXHiLfUJK7xi5KU8B7goPHF/VCe/V7Y2c4afyfgY2ePw4LxSDkCYNgYwqjSwGIbcsqv5ZRazBdDxR09ri6PknNyBVGLi5RlPXIrGQ3psNuf55qwxJxLO31qCZuvktKY5YvuIR4JPmBhuSFXOnn0ZiQw8uxMcQ0QA2lz+PxWCVNk9q+31H5DH1oYZDLfU3mijIOA+AJGZbBb+ZwBmpVL0+2TXLxE74WowdKEV+WTsKojNTd0VwcuRKRKR/6ynXAAis21y1X7Ui9FJE6mDIylUD40WXOKGJ1lYY41kRnYhVhvXYN9JtNYdY3HsCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFOnlASVD9fu3TAjptlW/gAXA4ql+MA0GCSqGSIb3DQEBCwUAA4IBAQCIpyRiChxp97crKfQ24Jt7z8P+AGpLf3sX4eL87ESa7QRoVJtXLmaut1pUEoYLQruKmh/0YFtZG9WxVgY6iuKbWnu7bOeMB/Ir+V/yrX3R+XvZOsuXiJnEbJiBW6lJzLldoW4f/71H+j1WD4tHpqmdMxq/sLqXfPIuc0/m0yFCn+ADBWGGB8Nn66vxtv+cT6p+RIVotXPQWbMilWp6pd5wSuB68FqrDwtYLNJtPwFs9MPVkuaJdYZ0eWd/rMcKD94Hgf89gvA0+qzMVFf+3BemXskjQRYy6CKsqoyC6jX4nhYjumAP/7psbzITsnpHtfCEEU+2JZwgM406aiMcsgLb","RawTBSCertificate":"MIIB1aADAgECAgEAMA0GCSqGSIb3DQEBCwUAMBgxFjAUBgNVBAMMDSoua3l2ZXJuby5zdmMwHhcNMjIwMTExMTMyNjQzWhcNMjMwMTExMTQyNjQzWjAYMRYwFAYDVQQDDA0qLmt5dmVybm8uc3ZjMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAywDPzn7KKej4yZ0qxW0fA2LegBWO6bNceIt9QkrvGLkpTwHuCg8cX9UJ79XtjZzhp/J+BjZ4/DgvFIOQJg2BjCqNLAYhtyyq/llFrMF0PFHT2uLo+Sc3IFUYuLlGU9cisZDemw25/nmrDEnEs7fWoJm6+S0pjli+4hHgk+YGG5IVc6efRmJDDy7ExxDRADaXP4/FYJU2T2r7fUfkMfWhhkMt9TeaKMg4D4AkZlsFv5nAGalUvT7ZNcvETvhajB0oRX5ZOwqiM1N3RXBy5EpEpH/rKdcACKzbXLVftSL0UkTqYMjKVQPjRZc4oYnWVhjjWRGdiFWG9dg30m01h1jcewIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQU6eUBJUP1+7dMCOm2Vb+ABcDiqX4=","RawSubjectPublicKeyInfo":"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAywDPzn7KKej4yZ0qxW0fA2LegBWO6bNceIt9QkrvGLkpTwHuCg8cX9UJ79XtjZzhp/J+BjZ4/DgvFIOQJg2BjCqNLAYhtyyq/llFrMF0PFHT2uLo+Sc3IFUYuLlGU9cisZDemw25/nmrDEnEs7fWoJm6+S0pjli+4hHgk+YGG5IVc6efRmJDDy7ExxDRADaXP4/FYJU2T2r7fUfkMfWhhkMt9TeaKMg4D4AkZlsFv5nAGalUvT7ZNcvETvhajB0oRX5ZOwqiM1N3RXBy5EpEpH/rKdcACKzbXLVftSL0UkTqYMjKVQPjRZc4oYnWVhjjWRGdiFWG9dg30m01h1jcewIDAQAB","RawSubject":"MBgxFjAUBgNVBAMMDSoua3l2ZXJuby5zdmM=","RawIssuer":"MBgxFjAUBgNVBAMMDSoua3l2ZXJuby5zdmM=","Signature":"iKckYgocafe3Kyn0NuCbe8/D/gBqS397F+Hi/OxEmu0EaFSbVy5mrrdaVBKGC0K7ipof9GBbWRvVsVYGOorim1p7u2znjAfyK/lf8q190fl72TrLl4iZxGyYgVupScy5XaFuH/+9R/o9Vg+LR6apnTMav7C6l3zyLnNP5tMhQp/gAwVhhgfDZ+ur8bb/nE+qfkSFaLVz0FmzIpVqeqXecErgevBaqw8LWCzSbT8BbPTD1ZLmiXWGdHlnf6zHCg/eB4H/PYLwNPqszFRX/twXpl7JI0EWMugirKqMguo1+J4WI7pgD/+6bG8yE7J6R7XwhBFPtiWcIDONOmojHLIC2w==","SignatureAlgorithm":4,"PublicKeyAlgorithm":1,"PublicKey":{"N":"25626776194299809103943925293022478779550111351090439168995035251396620593900589237452364135475983088162735720467798166985191488213022186077349821145857402701723499012699772423162550319145896535355752944351742979794245171828792388153331005254201525593934122190716637483002316913539755904599370968007653484768793099970920881706651907943367212661888776583428009130496820305182341702970575924538413569026902195901329094514102681440057150490032724791460671006772434362132998853498175356133386237155854830546292463707783883111067332118558636600306550854546869660051077649500890548685566726464348535891964886136890236394619","E":65537},"Version":3,"SerialNumber":0,"Issuer":{"Country":null,"Organization":null,"OrganizationalUnit":null,"Locality":null,"Province":null,"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"*.kyverno.svc","Names":[{"Type":[2,5,4,3],"Value":"*.kyverno.svc"}],"ExtraNames":null},"Subject":{"Country":null,"Organization":null,"OrganizationalUnit":null,"Locality":null,"Province":null,"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"*.kyverno.svc","Names":[{"Type":[2,5,4,3],"Value":"*.kyverno.svc"}],"ExtraNames":null},"NotBefore":"2022-01-11T13:26:43Z","NotAfter":"2023-01-11T14:26:43Z","KeyUsage":37,"Extensions":[{"Id":[2,5,29,15],"Critical":true,"Value":"AwICpA=="},{"Id":[2,5,29,19],"Critical":true,"Value":"MAMBAf8="},{"Id":[2,5,29,14],"Critical":false,"Value":"BBTp5QElQ/X7t0wI6bZVv4AFwOKpfg=="}],"ExtraExtensions":null,"UnhandledCriticalExtensions":null,"ExtKeyUsage":null,"UnknownExtKeyUsage":null,"BasicConstraintsValid":true,"IsCA":true,"MaxPathLen":-1,"MaxPathLenZero":false,"SubjectKeyId":"6eUBJUP1+7dMCOm2Vb+ABcDiqX4=","AuthorityKeyId":null,"OCSPServer":null,"IssuingCertificateURL":null,"DNSNames":null,"EmailAddresses":null,"IPAddresses":null,"URIs":null,"PermittedDNSDomainsCritical":false,"PermittedDNSDomains":null,"ExcludedDNSDomains":null,"PermittedIPRanges":null,"ExcludedIPRanges":null,"PermittedEmailAddresses":null,"ExcludedEmailAddresses":null,"PermittedURIDomains":null,"ExcludedURIDomains":null,"CRLDistributionPoints":null,"PolicyIdentifiers":null}`,
		`{"Raw":"MIIDSjCCAjKgAwIBAgIUWxmj40l+TDVJq98Xy7c6Leo3np8wDQYJKoZIhvcNAQELBQAwPTELMAkGA1UEBhMCeHgxCjAIBgNVBAgTAXgxCjAIBgNVBAcTAXgxCjAIBgNVBAoTAXgxCjAIBgNVBAsTAXgwHhcNMTgwMjAyMTIzODAwWhcNMjMwMjAxMTIzODAwWjA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UEChMBeDEKMAgGA1UECxMBeDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANHkqOmVf23KMXdaZU2eFUx1h4wb09JINBB8x/HL7UE0KFJcnOoVnNQB0gRukUopiYCzrzMFyGWWmB/pAEKool+ZiI2uMy6mcYBDtOi4pOm7U0TQQMV6L/5Yfi65xRz3RTMd/tYAoFi4aCZbJAGjxU6UWNYDzTy8E/cP6ZnlNbVHRiA6/wHsoWcXtWTXYP5yn9cf7EWQi1hOBM4BWmOIyB1f6LEgQipZWMOMPPHO3hsuSBn0rk7jovSt5XTlbgRrtxqAJiNjJUykWzIF+lLnZCioippGv5vkdGvE83JoACXvZTUwzA+MLu49fkw3bweqkbhrer8kacjfGlw3aJN37eECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFKXcb52bv6oqnD+D9fTNFHZL8IWxMA0GCSqGSIb3DQEBCwUAA4IBAQADvKvv3ym0XAYwKxPLLl3Lc6sJYHDbTN0donduG7PXeb1dhuukJ2lfufUYp2IGSAxuLecTYeeByOVp1gaMb5LsIGt2BVDmlMMkiH29LUHsvbyi85CpJo7A5RJG6AWW2VBCiDjz5v8JFM6pMkBRFfXH+pwIge65CE+MTSQcfb1/aIIoQ226P7E/3uUGX4k4pDXG/O7GNvykF40v1DB5y7DDBTQ4JWiJfyGkT69TmdOGLFAmjwxUjWyvEey4qJex/EGEm5RQcMv9iy7tba1wK7sykNGn5uDELGPGIIEAa5rIHm1FUFOZZVoELaasWS559wy8og39Eq21dDMynb8Bndn/","RawTBSCertificate":"MIICMqADAgECAhRbGaPjSX5MNUmr3xfLtzot6jeenzANBgkqhkiG9w0BAQsFADA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UEChMBeDEKMAgGA1UECxMBeDAeFw0xODAyMDIxMjM4MDBaFw0yMzAyMDExMjM4MDBaMD0xCzAJBgNVBAYTAnh4MQowCAYDVQQIEwF4MQowCAYDVQQHEwF4MQowCAYDVQQKEwF4MQowCAYDVQQLEwF4MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0eSo6ZV/bcoxd1plTZ4VTHWHjBvT0kg0EHzH8cvtQTQoUlyc6hWc1AHSBG6RSimJgLOvMwXIZZaYH+kAQqiiX5mIja4zLqZxgEO06Lik6btTRNBAxXov/lh+LrnFHPdFMx3+1gCgWLhoJlskAaPFTpRY1gPNPLwT9w/pmeU1tUdGIDr/AeyhZxe1ZNdg/nKf1x/sRZCLWE4EzgFaY4jIHV/osSBCKllYw4w88c7eGy5IGfSuTuOi9K3ldOVuBGu3GoAmI2MlTKRbMgX6UudkKKiKmka/m+R0a8TzcmgAJe9lNTDMD4wu7j1+TDdvB6qRuGt6vyRpyN8aXDdok3ft4QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUpdxvnZu/qiqcP4P19M0UdkvwhbE=","RawSubjectPublicKeyInfo":"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0eSo6ZV/bcoxd1plTZ4VTHWHjBvT0kg0EHzH8cvtQTQoUlyc6hWc1AHSBG6RSimJgLOvMwXIZZaYH+kAQqiiX5mIja4zLqZxgEO06Lik6btTRNBAxXov/lh+LrnFHPdFMx3+1gCgWLhoJlskAaPFTpRY1gPNPLwT9w/pmeU1tUdGIDr/AeyhZxe1ZNdg/nKf1x/sRZCLWE4EzgFaY4jIHV/osSBCKllYw4w88c7eGy5IGfSuTuOi9K3ldOVuBGu3GoAmI2MlTKRbMgX6UudkKKiKmka/m+R0a8TzcmgAJe9lNTDMD4wu7j1+TDdvB6qRuGt6vyRpyN8aXDdok3ft4QIDAQAB","RawSubject":"MD0xCzAJBgNVBAYTAnh4MQowCAYDVQQIEwF4MQowCAYDVQQHEwF4MQowCAYDVQQKEwF4MQowCAYDVQQLEwF4","RawIssuer":"MD0xCzAJBgNVBAYTAnh4MQowCAYDVQQIEwF4MQowCAYDVQQHEwF4MQowCAYDVQQKEwF4MQowCAYDVQQLEwF4","Signature":"A7yr798ptFwGMCsTyy5dy3OrCWBw20zdHaJ3bhuz13m9XYbrpCdpX7n1GKdiBkgMbi3nE2HngcjladYGjG+S7CBrdgVQ5pTDJIh9vS1B7L28ovOQqSaOwOUSRugFltlQQog48+b/CRTOqTJAURX1x/qcCIHuuQhPjE0kHH29f2iCKENtuj+xP97lBl+JOKQ1xvzuxjb8pBeNL9QwecuwwwU0OCVoiX8hpE+vU5nThixQJo8MVI1srxHsuKiXsfxBhJuUUHDL/Ysu7W2tcCu7MpDRp+bgxCxjxiCBAGuayB5tRVBTmWVaBC2mrFkuefcMvKIN/RKttXQzMp2/AZ3Z/w==","SignatureAlgorithm":4,"PublicKeyAlgorithm":1,"PublicKey":{"N":"26496562094779491076553211809422098021949952483515703281510813808490953126660362388109632773224118754702902108388229193869554055094778177099185065933983949693842239539154549752097759985799130804083586220803335221114269832081649712810220640441076536231140807229028981655981643835428138719795509959624793308640711388215921808921435203036357847686892066058381787405708754578605922703585581205444932036212009496723589206933777338978604488048677611723712498345752655171502746679687404543368933776929978831813434358099337112479727796701588293884856604804625411358577626503349165930794262171211166398339413648296787152727521","E":65537},"Version":3,"SerialNumber":520089955419326249038486015063014459614455897759,"Issuer":{"Country":["xx"],"Organization":["x"],"OrganizationalUnit":["x"],"Locality":["x"],"Province":["x"],"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"","Names":[{"Type":[2,5,4,6],"Value":"xx"},{"Type":[2,5,4,8],"Value":"x"},{"Type":[2,5,4,7],"Value":"x"},{"Type":[2,5,4,10],"Value":"x"},{"Type":[2,5,4,11],"Value":"x"}],"ExtraNames":null},"Subject":{"Country":["xx"],"Organization":["x"],"OrganizationalUnit":["x"],"Locality":["x"],"Province":["x"],"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"","Names":[{"Type":[2,5,4,6],"Value":"xx"},{"Type":[2,5,4,8],"Value":"x"},{"Type":[2,5,4,7],"Value":"x"},{"Type":[2,5,4,10],"Value":"x"},{"Type":[2,5,4,11],"Value":"x"}],"ExtraNames":null},"NotBefore":"2018-02-02T12:38:00Z","NotAfter":"2023-02-01T12:38:00Z","KeyUsage":96,"Extensions":[{"Id":[2,5,29,15],"Critical":true,"Value":"AwIBBg=="},{"Id":[2,5,29,19],"Critical":true,"Value":"MAMBAf8="},{"Id":[2,5,29,14],"Critical":false,"Value":"BBSl3G+dm7+qKpw/g/X0zRR2S/CFsQ=="}],"ExtraExtensions":null,"UnhandledCriticalExtensions":null,"ExtKeyUsage":null,"UnknownExtKeyUsage":null,"BasicConstraintsValid":true,"IsCA":true,"MaxPathLen":-1,"MaxPathLenZero":false,"SubjectKeyId":"pdxvnZu/qiqcP4P19M0UdkvwhbE=","AuthorityKeyId":null,"OCSPServer":null,"IssuingCertificateURL":null,"DNSNames":null,"EmailAddresses":null,"IPAddresses":null,"URIs":null,"PermittedDNSDomainsCritical":false,"PermittedDNSDomains":null,"ExcludedDNSDomains":null,"PermittedIPRanges":null,"ExcludedIPRanges":null,"PermittedEmailAddresses":null,"ExcludedEmailAddresses":null,"PermittedURIDomains":null,"ExcludedURIDomains":null,"CRLDistributionPoints":null,"PolicyIdentifiers":null}`,
	}
	resExpected := make([]map[string]interface{}, 2)

	for i, v := range resList {
		err := json.Unmarshal([]byte(v), &resExpected[i])
		assert.NilError(t, err)
	}

	testCases := []struct {
		jmesPath       string
		expectedResult map[string]interface{}
	}{
		{
			jmesPath:       "x509_decode(base64_decode('" + certs[0] + "'))",
			expectedResult: resExpected[0],
		},
		{
			jmesPath:       "x509_decode(base64_decode('" + certs[1] + "'))",
			expectedResult: resExpected[1],
		},
		{
			jmesPath:       "x509_decode('" + certs[2] + "')",
			expectedResult: resExpected[0],
		},
		{
			jmesPath:       "x509_decode('" + certs[3] + "')",
			expectedResult: resExpected[1],
		},
		{
			jmesPath:       "x509_decode('" + certs[4] + "')",
			expectedResult: resExpected[0],
		},
		{
			jmesPath:       "x509_decode('" + certs[5] + "')",
			expectedResult: resExpected[1],
		},
		{
			jmesPath:       "x509_decode('xyz')",
			expectedResult: map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := New(tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if err != nil && err.Error() != "invalid certificate" {
				assert.NilError(t, err)
			}

			res, ok := result.(map[string]interface{})
			assert.Assert(t, ok)
			assert.DeepEqual(t, res, tc.expectedResult)
		})
	}
}
