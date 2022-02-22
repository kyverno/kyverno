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
