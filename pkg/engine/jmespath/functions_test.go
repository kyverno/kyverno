package jmespath

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"gotest.tools/assert"
)

var cfg = config.NewDefaultConfiguration(false)

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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
			jp, err := newJMESPath(cfg, fmt.Sprintf(`to_string(parse_json('%s'))`, tc))
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
			jp, err := newJMESPath(cfg, tc.input)
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
			jp, err := newJMESPath(cfg, fmt.Sprintf(`parse_yaml('%s')`, tc.input))
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
	jp, err := newJMESPath(cfg, "replace_all('Lorem ipsum dolor sit amet', 'ipsum', 'muspi')")
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
	jp, err := newJMESPath(cfg, "trim('¡¡¡Hello, Gophers!!!', '!¡')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	trim, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, trim, "Hello, Gophers")
}

func Test_TrimPrefix(t *testing.T) {
	type args struct {
		arguments []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "trims prefix",
			args: args{
				arguments: []interface{}{"¡¡¡Hello, Gophers!!!", "¡¡¡Hello, "},
			},
			want:    "Gophers!!!",
			wantErr: false,
		},
		{
			name: "does not trim prefix",
			args: args{
				arguments: []interface{}{"¡¡¡Hello, Gophers!!!", "¡¡¡Hola, "},
			},
			want:    "¡¡¡Hello, Gophers!!!",
			wantErr: false,
		},
		{
			name: "invalid first argument",
			args: args{
				arguments: []interface{}{1, "¡¡¡Hello, "},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid first argument",
			args: args{
				arguments: []interface{}{"¡¡¡Hello, Gophers!!!", 1},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfTrimPrefix(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfTrimPrefix() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfTrimPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Split(t *testing.T) {
	jp, err := newJMESPath(cfg, "split('Hello, Gophers', ', ')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.([]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, split[0], "Hello")
	assert.Equal(t, split[1], "Gophers")
}

func Test_HasPrefix(t *testing.T) {
	jp, err := newJMESPath(cfg, "starts_with('Gophers', 'Go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Equal(t, split, true)
}

func Test_HasSuffix(t *testing.T) {
	jp, err := newJMESPath(cfg, "ends_with('Amigo', 'go')")
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

	query, err := newJMESPath(cfg, "regex_match('12.*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_RegexMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := newJMESPath(cfg, "regex_match('12.*', abs(foo))")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_PatternMatch(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = "prefix-foo"

	query, err := newJMESPath(cfg, "pattern_match('prefix-*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_PatternMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := newJMESPath(cfg, "pattern_match('12*', abs(foo))")
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
	query, err := newJMESPath(cfg, `regex_replace_all('([Hh]e|G)l', spec.field, '${2}G')`)
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

	query, err := newJMESPath(cfg, `regex_replace_all_literal('[Hh]el?', spec.field, 'G')`)
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

			query, err := newJMESPath(cfg, "label_match(`"+tc.test+"`, metadata.labels)")
			assert.NilError(t, err)

			res, err := query.Search(resource)
			assert.NilError(t, err)

			result, ok := res.(bool)
			assert.Assert(t, ok)

			assert.Equal(t, result, tc.expectedResult)
		})
	}
}

func Test_JpToBoolean(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected interface{}
		err      bool
	}{
		{"true", true, false},
		{"TRue", true, false},
		{"FaLse", false, false},
		{"FaLsee", nil, true},
		{"false", false, false},
		{"foo", nil, true},
		{1, nil, true},
		{nil, nil, true},
	}
	for _, tc := range testCases {
		res, err := jpToBoolean([]interface{}{tc.input})
		if tc.err && err == nil {
			t.Errorf("Expected an error but received nil")
		}
		if !tc.err && err != nil {
			t.Errorf("Expected nil error but received: %s", err)
		}
		if res != tc.expected {
			t.Errorf("Expected %v but received %v", tc.expected, res)
		}
	}
}

func Test_Base64Decode(t *testing.T) {
	jp, err := newJMESPath(cfg, "base64_decode('SGVsbG8sIHdvcmxkIQ==')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	str, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, str, "Hello, world!")
}

func Test_Base64Encode(t *testing.T) {
	jp, err := newJMESPath(cfg, "base64_encode('Hello, world!')")
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
	}`)
	var resource interface{}
	err := json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)

	query, err := newJMESPath(cfg, `base64_decode(data.example1)`)
	assert.NilError(t, err)

	res, err := query.Search(resource)
	assert.NilError(t, err)

	result, ok := res.(string)
	assert.Assert(t, ok)
	assert.Equal(t, string(result), "Hello, world!")
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
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
			jp, err := newJMESPath(cfg, tc.jmesPath)
			assert.NilError(t, err)

			result, err := jp.Search("")
			assert.NilError(t, err)

			res, ok := result.(bool)
			assert.Assert(t, ok)
			assert.Equal(t, res, tc.expectedResult)
		})
	}
}

func Test_Lookup(t *testing.T) {
	testCases := []struct {
		collection     string
		key            string
		expectedResult string
	}{
		// objects
		/////////////////

		// not found
		{
			collection:     `{}`,
			key:            `"key1"`,
			expectedResult: `null`,
		},
		{
			collection:     `{"key1": "value1"}`,
			key:            `"key2"`,
			expectedResult: `null`,
		},

		// found
		{
			collection:     `{"key1": "value1"}`,
			key:            `"key1"`,
			expectedResult: `"value1"`,
		},
		{
			collection:     `{"": "value1"}`,
			key:            `""`,
			expectedResult: `"value1"`,
		},

		// result types
		{
			collection:     `{"k": 123}`,
			key:            `"k"`,
			expectedResult: `123`,
		},
		{
			collection:     `{"k": 12.34}`,
			key:            `"k"`,
			expectedResult: `12.34`,
		},
		{
			collection:     `{"k": true}`,
			key:            `"k"`,
			expectedResult: `true`,
		},
		{
			collection:     `{"k": false}`,
			key:            `"k"`,
			expectedResult: `false`,
		},
		{
			collection:     `{"k": null}`,
			key:            `"k"`,
			expectedResult: `null`,
		},
		{
			collection:     `{"k":  [7, "x", true] }`,
			key:            `"k"`,
			expectedResult: `[7, "x", true]`,
		},
		{
			collection:     `{"k": {"key1":true}}`,
			key:            `"k"`,
			expectedResult: `{"key1":true}`,
		},

		// arrays
		/////////////////

		// not found
		{
			collection:     `[]`,
			key:            `0`,
			expectedResult: `null`,
		},
		{
			collection:     `["item0"]`,
			key:            `-1`,
			expectedResult: `null`,
		},
		{
			collection:     `["item0"]`,
			key:            `1`,
			expectedResult: `null`,
		},

		// found
		{
			collection:     `["item0"]`,
			key:            `0`,
			expectedResult: `"item0"`,
		},
		{
			collection:     `["item0", "item1", "item2", "item3"]`,
			key:            `2`,
			expectedResult: `"item2"`,
		},
		{
			collection:     `["item0", "item1"]`,
			key:            `0.99999999999999999999999999999999999999999999`,
			expectedResult: `"item1"`,
		},

		// result types
		{
			collection:     `[123]`,
			key:            `0`,
			expectedResult: `123`,
		},
		{
			collection:     `[12.34]`,
			key:            `0`,
			expectedResult: `12.34`,
		},
		{
			collection:     `[true]`,
			key:            `0`,
			expectedResult: `true`,
		},
		{
			collection:     `[false]`,
			key:            `0`,
			expectedResult: `false`,
		},
		{
			collection:     `[null]`,
			key:            `0`,
			expectedResult: `null`,
		},
		{
			collection:     `[ [7, "x", true] ]`,
			key:            `0`,
			expectedResult: `[7, "x", true]`,
		},
		{
			collection:     `[{"key1":true}]`,
			key:            `0`,
			expectedResult: `{"key1":true}`,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, "lookup(`"+tc.collection+"`,`"+tc.key+"`)")
			assert.NilError(t, err)

			result, err := query.Search("")
			assert.NilError(t, err)

			var expectedResult interface{}
			err = json.Unmarshal([]byte(tc.expectedResult), &expectedResult)
			assert.NilError(t, err)

			assert.DeepEqual(t, result, expectedResult)
		})
	}
}

func Test_Lookup_InvalidArgs(t *testing.T) {
	testCases := []struct {
		collection  string
		key         string
		expectedMsg string
	}{
		// invalid key type
		{
			collection:  `{}`,
			key:         `123`,
			expectedMsg: `argument #2 is not of type String`,
		},
		{
			collection:  `[]`,
			key:         `"abc"`,
			expectedMsg: `argument #2 is not of type Number`,
		},

		// invalid value
		{
			collection:  `[]`,
			key:         `1.5`,
			expectedMsg: `JMESPath function 'lookup': argument #2: expected an integer number but got: 1.5`,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, "lookup(`"+tc.collection+"`,`"+tc.key+"`)")
			assert.NilError(t, err)

			_, err = query.Search("")
			assert.ErrorContains(t, err, tc.expectedMsg)
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
		{
			object:         `["A", "B", "C"]`,
			keyName:        `"myKey"`,
			valName:        `"myValue"`,
			expectedResult: `[{ "myKey": 0, "myValue": "A" }, { "myKey": 1, "myValue": "B" }, { "myKey": 2, "myValue": "C" }]`,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			query, err := newJMESPath(cfg, "items(`"+tc.object+"`,`"+tc.keyName+"`,`"+tc.valName+"`)")
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
			query, err := newJMESPath(cfg, "object_from_lists(`"+tc.keys+"`,`"+tc.values+"`)")
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
		`-----BEGIN CERTIFICATE REQUEST-----
MIICvDCCAaQCAQAwdzELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFV0YWgxDzANBgNV
BAcMBkxpbmRvbjEWMBQGA1UECgwNRGlnaUNlcnQgSW5jLjERMA8GA1UECwwIRGln
aUNlcnQxHTAbBgNVBAMMFGV4YW1wbGUuZGlnaWNlcnQuY29tMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEA8+To7d+2kPWeBv/orU3LVbJwDrSQbeKamCmo
wp5bqDxIwV20zqRb7APUOKYoVEFFOEQs6T6gImnIolhbiH6m4zgZ/CPvWBOkZc+c
1Po2EmvBz+AD5sBdT5kzGQA6NbWyZGldxRthNLOs1efOhdnWFuhI162qmcflgpiI
WDuwq4C9f+YkeJhNn9dF5+owm8cOQmDrV8NNdiTqin8q3qYAHHJRW28glJUCZkTZ
wIaSR6crBQ8TbYNE0dc+Caa3DOIkz1EOsHWzTx+n0zKfqcbgXi4DJx+C1bjptYPR
BPZL8DAeWuA8ebudVT44yEp82G96/Ggcf7F33xMxe0yc+Xa6owIDAQABoAAwDQYJ
KoZIhvcNAQEFBQADggEBAB0kcrFccSmFDmxox0Ne01UIqSsDqHgL+XmHTXJwre6D
hJSZwbvEtOK0G3+dr4Fs11WuUNt5qcLsx5a8uk4G6AKHMzuhLsJ7XZjgmQXGECpY
Q4mC3yT3ZoCGpIXbw+iP3lmEEXgaQL0Tx5LFl/okKbKYwIqNiyKWOMj7ZR/wxWg/
ZDGRs55xuoeLDJ/ZRFf9bI+IaCUd1YrfYcHIl3G87Av+r49YVwqRDT0VDV7uLgqn
29XI1PpVUNCPQGn9p/eX6Qo7vpDaPybRtA2R7XLKjQaF9oXWeCUqy1hvJac9QFO2
97Ob1alpHPoZ7mWiEuJwjBPii6a9M9G30nUo39lBi1w=
-----END CERTIFICATE REQUEST-----`,
	}
	resList := []string{
		`{"Raw":"MIIC7TCCAdWgAwIBAgIBADANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDDA0qLmt5dmVybm8uc3ZjMB4XDTIyMDExMTEzMjY0M1oXDTIzMDExMTE0MjY0M1owGDEWMBQGA1UEAwwNKi5reXZlcm5vLnN2YzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMsAz85+yino+MmdKsVtHwNi3oAVjumzXHiLfUJK7xi5KU8B7goPHF/VCe/V7Y2c4afyfgY2ePw4LxSDkCYNgYwqjSwGIbcsqv5ZRazBdDxR09ri6PknNyBVGLi5RlPXIrGQ3psNuf55qwxJxLO31qCZuvktKY5YvuIR4JPmBhuSFXOnn0ZiQw8uxMcQ0QA2lz+PxWCVNk9q+31H5DH1oYZDLfU3mijIOA+AJGZbBb+ZwBmpVL0+2TXLxE74WowdKEV+WTsKojNTd0VwcuRKRKR/6ynXAAis21y1X7Ui9FJE6mDIylUD40WXOKGJ1lYY41kRnYhVhvXYN9JtNYdY3HsCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFOnlASVD9fu3TAjptlW/gAXA4ql+MA0GCSqGSIb3DQEBCwUAA4IBAQCIpyRiChxp97crKfQ24Jt7z8P+AGpLf3sX4eL87ESa7QRoVJtXLmaut1pUEoYLQruKmh/0YFtZG9WxVgY6iuKbWnu7bOeMB/Ir+V/yrX3R+XvZOsuXiJnEbJiBW6lJzLldoW4f/71H+j1WD4tHpqmdMxq/sLqXfPIuc0/m0yFCn+ADBWGGB8Nn66vxtv+cT6p+RIVotXPQWbMilWp6pd5wSuB68FqrDwtYLNJtPwFs9MPVkuaJdYZ0eWd/rMcKD94Hgf89gvA0+qzMVFf+3BemXskjQRYy6CKsqoyC6jX4nhYjumAP/7psbzITsnpHtfCEEU+2JZwgM406aiMcsgLb","RawTBSCertificate":"MIIB1aADAgECAgEAMA0GCSqGSIb3DQEBCwUAMBgxFjAUBgNVBAMMDSoua3l2ZXJuby5zdmMwHhcNMjIwMTExMTMyNjQzWhcNMjMwMTExMTQyNjQzWjAYMRYwFAYDVQQDDA0qLmt5dmVybm8uc3ZjMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAywDPzn7KKej4yZ0qxW0fA2LegBWO6bNceIt9QkrvGLkpTwHuCg8cX9UJ79XtjZzhp/J+BjZ4/DgvFIOQJg2BjCqNLAYhtyyq/llFrMF0PFHT2uLo+Sc3IFUYuLlGU9cisZDemw25/nmrDEnEs7fWoJm6+S0pjli+4hHgk+YGG5IVc6efRmJDDy7ExxDRADaXP4/FYJU2T2r7fUfkMfWhhkMt9TeaKMg4D4AkZlsFv5nAGalUvT7ZNcvETvhajB0oRX5ZOwqiM1N3RXBy5EpEpH/rKdcACKzbXLVftSL0UkTqYMjKVQPjRZc4oYnWVhjjWRGdiFWG9dg30m01h1jcewIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQU6eUBJUP1+7dMCOm2Vb+ABcDiqX4=","RawSubjectPublicKeyInfo":"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAywDPzn7KKej4yZ0qxW0fA2LegBWO6bNceIt9QkrvGLkpTwHuCg8cX9UJ79XtjZzhp/J+BjZ4/DgvFIOQJg2BjCqNLAYhtyyq/llFrMF0PFHT2uLo+Sc3IFUYuLlGU9cisZDemw25/nmrDEnEs7fWoJm6+S0pjli+4hHgk+YGG5IVc6efRmJDDy7ExxDRADaXP4/FYJU2T2r7fUfkMfWhhkMt9TeaKMg4D4AkZlsFv5nAGalUvT7ZNcvETvhajB0oRX5ZOwqiM1N3RXBy5EpEpH/rKdcACKzbXLVftSL0UkTqYMjKVQPjRZc4oYnWVhjjWRGdiFWG9dg30m01h1jcewIDAQAB","RawSubject":"MBgxFjAUBgNVBAMMDSoua3l2ZXJuby5zdmM=","RawIssuer":"MBgxFjAUBgNVBAMMDSoua3l2ZXJuby5zdmM=","Signature":"iKckYgocafe3Kyn0NuCbe8/D/gBqS397F+Hi/OxEmu0EaFSbVy5mrrdaVBKGC0K7ipof9GBbWRvVsVYGOorim1p7u2znjAfyK/lf8q190fl72TrLl4iZxGyYgVupScy5XaFuH/+9R/o9Vg+LR6apnTMav7C6l3zyLnNP5tMhQp/gAwVhhgfDZ+ur8bb/nE+qfkSFaLVz0FmzIpVqeqXecErgevBaqw8LWCzSbT8BbPTD1ZLmiXWGdHlnf6zHCg/eB4H/PYLwNPqszFRX/twXpl7JI0EWMugirKqMguo1+J4WI7pgD/+6bG8yE7J6R7XwhBFPtiWcIDONOmojHLIC2w==","SignatureAlgorithm":4,"PublicKeyAlgorithm":1,"PublicKey":{"N":"25626776194299809103943925293022478779550111351090439168995035251396620593900589237452364135475983088162735720467798166985191488213022186077349821145857402701723499012699772423162550319145896535355752944351742979794245171828792388153331005254201525593934122190716637483002316913539755904599370968007653484768793099970920881706651907943367212661888776583428009130496820305182341702970575924538413569026902195901329094514102681440057150490032724791460671006772434362132998853498175356133386237155854830546292463707783883111067332118558636600306550854546869660051077649500890548685566726464348535891964886136890236394619","E":65537},"Version":3,"SerialNumber":0,"Issuer":{"Country":null,"Organization":null,"OrganizationalUnit":null,"Locality":null,"Province":null,"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"*.kyverno.svc","Names":[{"Type":[2,5,4,3],"Value":"*.kyverno.svc"}],"ExtraNames":null},"Subject":{"Country":null,"Organization":null,"OrganizationalUnit":null,"Locality":null,"Province":null,"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"*.kyverno.svc","Names":[{"Type":[2,5,4,3],"Value":"*.kyverno.svc"}],"ExtraNames":null},"NotBefore":"2022-01-11T13:26:43Z","NotAfter":"2023-01-11T14:26:43Z","KeyUsage":37,"Extensions":[{"Id":[2,5,29,15],"Critical":true,"Value":"AwICpA=="},{"Id":[2,5,29,19],"Critical":true,"Value":"MAMBAf8="},{"Id":[2,5,29,14],"Critical":false,"Value":"BBTp5QElQ/X7t0wI6bZVv4AFwOKpfg=="}],"ExtraExtensions":null,"UnhandledCriticalExtensions":null,"ExtKeyUsage":null,"UnknownExtKeyUsage":null,"BasicConstraintsValid":true,"IsCA":true,"MaxPathLen":-1,"MaxPathLenZero":false,"SubjectKeyId":"6eUBJUP1+7dMCOm2Vb+ABcDiqX4=","AuthorityKeyId":null,"OCSPServer":null,"IssuingCertificateURL":null,"DNSNames":null,"EmailAddresses":null,"IPAddresses":null,"URIs":null,"PermittedDNSDomainsCritical":false,"PermittedDNSDomains":null,"ExcludedDNSDomains":null,"PermittedIPRanges":null,"ExcludedIPRanges":null,"PermittedEmailAddresses":null,"ExcludedEmailAddresses":null,"PermittedURIDomains":null,"ExcludedURIDomains":null,"CRLDistributionPoints":null,"PolicyIdentifiers":null}`,
		`{"Raw":"MIIDSjCCAjKgAwIBAgIUWxmj40l+TDVJq98Xy7c6Leo3np8wDQYJKoZIhvcNAQELBQAwPTELMAkGA1UEBhMCeHgxCjAIBgNVBAgTAXgxCjAIBgNVBAcTAXgxCjAIBgNVBAoTAXgxCjAIBgNVBAsTAXgwHhcNMTgwMjAyMTIzODAwWhcNMjMwMjAxMTIzODAwWjA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UEChMBeDEKMAgGA1UECxMBeDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANHkqOmVf23KMXdaZU2eFUx1h4wb09JINBB8x/HL7UE0KFJcnOoVnNQB0gRukUopiYCzrzMFyGWWmB/pAEKool+ZiI2uMy6mcYBDtOi4pOm7U0TQQMV6L/5Yfi65xRz3RTMd/tYAoFi4aCZbJAGjxU6UWNYDzTy8E/cP6ZnlNbVHRiA6/wHsoWcXtWTXYP5yn9cf7EWQi1hOBM4BWmOIyB1f6LEgQipZWMOMPPHO3hsuSBn0rk7jovSt5XTlbgRrtxqAJiNjJUykWzIF+lLnZCioippGv5vkdGvE83JoACXvZTUwzA+MLu49fkw3bweqkbhrer8kacjfGlw3aJN37eECAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFKXcb52bv6oqnD+D9fTNFHZL8IWxMA0GCSqGSIb3DQEBCwUAA4IBAQADvKvv3ym0XAYwKxPLLl3Lc6sJYHDbTN0donduG7PXeb1dhuukJ2lfufUYp2IGSAxuLecTYeeByOVp1gaMb5LsIGt2BVDmlMMkiH29LUHsvbyi85CpJo7A5RJG6AWW2VBCiDjz5v8JFM6pMkBRFfXH+pwIge65CE+MTSQcfb1/aIIoQ226P7E/3uUGX4k4pDXG/O7GNvykF40v1DB5y7DDBTQ4JWiJfyGkT69TmdOGLFAmjwxUjWyvEey4qJex/EGEm5RQcMv9iy7tba1wK7sykNGn5uDELGPGIIEAa5rIHm1FUFOZZVoELaasWS559wy8og39Eq21dDMynb8Bndn/","RawTBSCertificate":"MIICMqADAgECAhRbGaPjSX5MNUmr3xfLtzot6jeenzANBgkqhkiG9w0BAQsFADA9MQswCQYDVQQGEwJ4eDEKMAgGA1UECBMBeDEKMAgGA1UEBxMBeDEKMAgGA1UEChMBeDEKMAgGA1UECxMBeDAeFw0xODAyMDIxMjM4MDBaFw0yMzAyMDExMjM4MDBaMD0xCzAJBgNVBAYTAnh4MQowCAYDVQQIEwF4MQowCAYDVQQHEwF4MQowCAYDVQQKEwF4MQowCAYDVQQLEwF4MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0eSo6ZV/bcoxd1plTZ4VTHWHjBvT0kg0EHzH8cvtQTQoUlyc6hWc1AHSBG6RSimJgLOvMwXIZZaYH+kAQqiiX5mIja4zLqZxgEO06Lik6btTRNBAxXov/lh+LrnFHPdFMx3+1gCgWLhoJlskAaPFTpRY1gPNPLwT9w/pmeU1tUdGIDr/AeyhZxe1ZNdg/nKf1x/sRZCLWE4EzgFaY4jIHV/osSBCKllYw4w88c7eGy5IGfSuTuOi9K3ldOVuBGu3GoAmI2MlTKRbMgX6UudkKKiKmka/m+R0a8TzcmgAJe9lNTDMD4wu7j1+TDdvB6qRuGt6vyRpyN8aXDdok3ft4QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUpdxvnZu/qiqcP4P19M0UdkvwhbE=","RawSubjectPublicKeyInfo":"MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0eSo6ZV/bcoxd1plTZ4VTHWHjBvT0kg0EHzH8cvtQTQoUlyc6hWc1AHSBG6RSimJgLOvMwXIZZaYH+kAQqiiX5mIja4zLqZxgEO06Lik6btTRNBAxXov/lh+LrnFHPdFMx3+1gCgWLhoJlskAaPFTpRY1gPNPLwT9w/pmeU1tUdGIDr/AeyhZxe1ZNdg/nKf1x/sRZCLWE4EzgFaY4jIHV/osSBCKllYw4w88c7eGy5IGfSuTuOi9K3ldOVuBGu3GoAmI2MlTKRbMgX6UudkKKiKmka/m+R0a8TzcmgAJe9lNTDMD4wu7j1+TDdvB6qRuGt6vyRpyN8aXDdok3ft4QIDAQAB","RawSubject":"MD0xCzAJBgNVBAYTAnh4MQowCAYDVQQIEwF4MQowCAYDVQQHEwF4MQowCAYDVQQKEwF4MQowCAYDVQQLEwF4","RawIssuer":"MD0xCzAJBgNVBAYTAnh4MQowCAYDVQQIEwF4MQowCAYDVQQHEwF4MQowCAYDVQQKEwF4MQowCAYDVQQLEwF4","Signature":"A7yr798ptFwGMCsTyy5dy3OrCWBw20zdHaJ3bhuz13m9XYbrpCdpX7n1GKdiBkgMbi3nE2HngcjladYGjG+S7CBrdgVQ5pTDJIh9vS1B7L28ovOQqSaOwOUSRugFltlQQog48+b/CRTOqTJAURX1x/qcCIHuuQhPjE0kHH29f2iCKENtuj+xP97lBl+JOKQ1xvzuxjb8pBeNL9QwecuwwwU0OCVoiX8hpE+vU5nThixQJo8MVI1srxHsuKiXsfxBhJuUUHDL/Ysu7W2tcCu7MpDRp+bgxCxjxiCBAGuayB5tRVBTmWVaBC2mrFkuefcMvKIN/RKttXQzMp2/AZ3Z/w==","SignatureAlgorithm":4,"PublicKeyAlgorithm":1,"PublicKey":{"N":"26496562094779491076553211809422098021949952483515703281510813808490953126660362388109632773224118754702902108388229193869554055094778177099185065933983949693842239539154549752097759985799130804083586220803335221114269832081649712810220640441076536231140807229028981655981643835428138719795509959624793308640711388215921808921435203036357847686892066058381787405708754578605922703585581205444932036212009496723589206933777338978604488048677611723712498345752655171502746679687404543368933776929978831813434358099337112479727796701588293884856604804625411358577626503349165930794262171211166398339413648296787152727521","E":65537},"Version":3,"SerialNumber":520089955419326249038486015063014459614455897759,"Issuer":{"Country":["xx"],"Organization":["x"],"OrganizationalUnit":["x"],"Locality":["x"],"Province":["x"],"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"","Names":[{"Type":[2,5,4,6],"Value":"xx"},{"Type":[2,5,4,8],"Value":"x"},{"Type":[2,5,4,7],"Value":"x"},{"Type":[2,5,4,10],"Value":"x"},{"Type":[2,5,4,11],"Value":"x"}],"ExtraNames":null},"Subject":{"Country":["xx"],"Organization":["x"],"OrganizationalUnit":["x"],"Locality":["x"],"Province":["x"],"StreetAddress":null,"PostalCode":null,"SerialNumber":"","CommonName":"","Names":[{"Type":[2,5,4,6],"Value":"xx"},{"Type":[2,5,4,8],"Value":"x"},{"Type":[2,5,4,7],"Value":"x"},{"Type":[2,5,4,10],"Value":"x"},{"Type":[2,5,4,11],"Value":"x"}],"ExtraNames":null},"NotBefore":"2018-02-02T12:38:00Z","NotAfter":"2023-02-01T12:38:00Z","KeyUsage":96,"Extensions":[{"Id":[2,5,29,15],"Critical":true,"Value":"AwIBBg=="},{"Id":[2,5,29,19],"Critical":true,"Value":"MAMBAf8="},{"Id":[2,5,29,14],"Critical":false,"Value":"BBSl3G+dm7+qKpw/g/X0zRR2S/CFsQ=="}],"ExtraExtensions":null,"UnhandledCriticalExtensions":null,"ExtKeyUsage":null,"UnknownExtKeyUsage":null,"BasicConstraintsValid":true,"IsCA":true,"MaxPathLen":-1,"MaxPathLenZero":false,"SubjectKeyId":"pdxvnZu/qiqcP4P19M0UdkvwhbE=","AuthorityKeyId":null,"OCSPServer":null,"IssuingCertificateURL":null,"DNSNames":null,"EmailAddresses":null,"IPAddresses":null,"URIs":null,"PermittedDNSDomainsCritical":false,"PermittedDNSDomains":null,"ExcludedDNSDomains":null,"PermittedIPRanges":null,"ExcludedIPRanges":null,"PermittedEmailAddresses":null,"ExcludedEmailAddresses":null,"PermittedURIDomains":null,"ExcludedURIDomains":null,"CRLDistributionPoints":null,"PolicyIdentifiers":null}`,
		`{"Attributes": null,"DNSNames": null,"EmailAddresses": null,"Extensions": null,"ExtraExtensions": null,"IPAddresses": null,"PublicKey": {"E": 65537,"N": "30788787775499084229626026724118719872973907471499649646184775670914346180312671906399223325409590948519743636184795333482381888453996128329396648505249062053283056069530767359210562374203250761551376585013181653210719557451154530514423713570995019036786795900989905655136970670786111875127185122973524433496741842862203002594125711406631836733656561027033024624302759714504708249269624951711291364305004897900464453081676928894280743798888738608709381777168414778329993619693869221517193116446955833233290395600921852333943656575398427367952052258926688943219100950267027328710138285403327192731641778165311310576291"},"PublicKeyAlgorithm": 1,"Raw": "MIICvDCCAaQCAQAwdzELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFV0YWgxDzANBgNVBAcMBkxpbmRvbjEWMBQGA1UECgwNRGlnaUNlcnQgSW5jLjERMA8GA1UECwwIRGlnaUNlcnQxHTAbBgNVBAMMFGV4YW1wbGUuZGlnaWNlcnQuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA8+To7d+2kPWeBv/orU3LVbJwDrSQbeKamCmowp5bqDxIwV20zqRb7APUOKYoVEFFOEQs6T6gImnIolhbiH6m4zgZ/CPvWBOkZc+c1Po2EmvBz+AD5sBdT5kzGQA6NbWyZGldxRthNLOs1efOhdnWFuhI162qmcflgpiIWDuwq4C9f+YkeJhNn9dF5+owm8cOQmDrV8NNdiTqin8q3qYAHHJRW28glJUCZkTZwIaSR6crBQ8TbYNE0dc+Caa3DOIkz1EOsHWzTx+n0zKfqcbgXi4DJx+C1bjptYPRBPZL8DAeWuA8ebudVT44yEp82G96/Ggcf7F33xMxe0yc+Xa6owIDAQABoAAwDQYJKoZIhvcNAQEFBQADggEBAB0kcrFccSmFDmxox0Ne01UIqSsDqHgL+XmHTXJwre6DhJSZwbvEtOK0G3+dr4Fs11WuUNt5qcLsx5a8uk4G6AKHMzuhLsJ7XZjgmQXGECpYQ4mC3yT3ZoCGpIXbw+iP3lmEEXgaQL0Tx5LFl/okKbKYwIqNiyKWOMj7ZR/wxWg/ZDGRs55xuoeLDJ/ZRFf9bI+IaCUd1YrfYcHIl3G87Av+r49YVwqRDT0VDV7uLgqn29XI1PpVUNCPQGn9p/eX6Qo7vpDaPybRtA2R7XLKjQaF9oXWeCUqy1hvJac9QFO297Ob1alpHPoZ7mWiEuJwjBPii6a9M9G30nUo39lBi1w=","RawSubject": "MHcxCzAJBgNVBAYTAlVTMQ0wCwYDVQQIDARVdGFoMQ8wDQYDVQQHDAZMaW5kb24xFjAUBgNVBAoMDURpZ2lDZXJ0IEluYy4xETAPBgNVBAsMCERpZ2lDZXJ0MR0wGwYDVQQDDBRleGFtcGxlLmRpZ2ljZXJ0LmNvbQ==","RawSubjectPublicKeyInfo": "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA8+To7d+2kPWeBv/orU3LVbJwDrSQbeKamCmowp5bqDxIwV20zqRb7APUOKYoVEFFOEQs6T6gImnIolhbiH6m4zgZ/CPvWBOkZc+c1Po2EmvBz+AD5sBdT5kzGQA6NbWyZGldxRthNLOs1efOhdnWFuhI162qmcflgpiIWDuwq4C9f+YkeJhNn9dF5+owm8cOQmDrV8NNdiTqin8q3qYAHHJRW28glJUCZkTZwIaSR6crBQ8TbYNE0dc+Caa3DOIkz1EOsHWzTx+n0zKfqcbgXi4DJx+C1bjptYPRBPZL8DAeWuA8ebudVT44yEp82G96/Ggcf7F33xMxe0yc+Xa6owIDAQAB","RawTBSCertificateRequest": "MIIBpAIBADB3MQswCQYDVQQGEwJVUzENMAsGA1UECAwEVXRhaDEPMA0GA1UEBwwGTGluZG9uMRYwFAYDVQQKDA1EaWdpQ2VydCBJbmMuMREwDwYDVQQLDAhEaWdpQ2VydDEdMBsGA1UEAwwUZXhhbXBsZS5kaWdpY2VydC5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDz5Ojt37aQ9Z4G/+itTctVsnAOtJBt4pqYKajCnluoPEjBXbTOpFvsA9Q4pihUQUU4RCzpPqAiaciiWFuIfqbjOBn8I+9YE6Rlz5zU+jYSa8HP4APmwF1PmTMZADo1tbJkaV3FG2E0s6zV586F2dYW6EjXraqZx+WCmIhYO7CrgL1/5iR4mE2f10Xn6jCbxw5CYOtXw012JOqKfyrepgAcclFbbyCUlQJmRNnAhpJHpysFDxNtg0TR1z4JprcM4iTPUQ6wdbNPH6fTMp+pxuBeLgMnH4LVuOm1g9EE9kvwMB5a4Dx5u51VPjjISnzYb3r8aBx/sXffEzF7TJz5drqjAgMBAAGgAA==","Signature": "HSRysVxxKYUObGjHQ17TVQipKwOoeAv5eYdNcnCt7oOElJnBu8S04rQbf52vgWzXVa5Q23mpwuzHlry6TgboAoczO6EuwntdmOCZBcYQKlhDiYLfJPdmgIakhdvD6I/eWYQReBpAvRPHksWX+iQpspjAio2LIpY4yPtlH/DFaD9kMZGznnG6h4sMn9lEV/1sj4hoJR3Vit9hwciXcbzsC/6vj1hXCpENPRUNXu4uCqfb1cjU+lVQ0I9Aaf2n95fpCju+kNo/JtG0DZHtcsqNBoX2hdZ4JSrLWG8lpz1AU7b3s5vVqWkc+hnuZaIS4nCME+KLpr0z0bfSdSjf2UGLXA==","SignatureAlgorithm": 3,"Subject": {"CommonName": "example.digicert.com","Country": ["US"],"ExtraNames": null,"Locality": ["Lindon"],"Names": [{"Type": [2,5,4,6],"Value": "US"},{"Type": [2,5,4,8],"Value": "Utah"},{"Type": [2,5,4,7],"Value": "Lindon"},{"Type": [2,5,4,10],"Value": "DigiCert Inc."},{"Type": [2,5,4,11],"Value": "DigiCert"},{"Type": [2,5,4,3],"Value": "example.digicert.com"}],"Organization": ["DigiCert Inc."],"OrganizationalUnit": ["DigiCert"],"PostalCode": null,"Province": ["Utah"],"SerialNumber": "","StreetAddress": null},"URIs": null,"Version": 0}`,
	}
	resExpected := make([]map[string]interface{}, 3)
	for i, v := range resList {
		err := json.Unmarshal([]byte(v), &resExpected[i])
		assert.NilError(t, err)
	}
	testCases := []struct {
		jmesPath       string
		expectedResult map[string]interface{}
	}{{
		jmesPath:       "x509_decode(base64_decode('" + certs[0] + "'))",
		expectedResult: resExpected[0],
	}, {
		jmesPath:       "x509_decode(base64_decode('" + certs[1] + "'))",
		expectedResult: resExpected[1],
	}, {
		jmesPath:       "x509_decode('" + certs[2] + "')",
		expectedResult: resExpected[0],
	}, {
		jmesPath:       "x509_decode('" + certs[3] + "')",
		expectedResult: resExpected[1],
	}, {
		jmesPath:       "x509_decode('" + certs[4] + "')",
		expectedResult: resExpected[0],
	}, {
		jmesPath:       "x509_decode('" + certs[5] + "')",
		expectedResult: resExpected[1],
	}, {
		jmesPath:       "x509_decode('" + certs[6] + "')",
		expectedResult: resExpected[2],
	},
	// {
	// 	jmesPath:       "x509_decode('xyz')",
	// 	expectedResult: map[string]interface{}{},
	// }
	}
	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.jmesPath)
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

func Test_jpfCompare(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{"a", "b"},
		},
		want: -1,
	}, {
		args: args{
			arguments: []interface{}{"b", "a"},
		},
		want: 1,
	}, {
		args: args{
			arguments: []interface{}{"b", "b"},
		},
		want: 0,
	}, {
		args: args{
			arguments: []interface{}{1, "b"},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{"a", 1},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfCompare(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfCompare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpfEqualFold(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{"Go", "go"},
		},
		want: true,
	}, {
		args: args{
			arguments: []interface{}{"a", "b"},
		},
		want: false,
	}, {
		args: args{
			arguments: []interface{}{1, "b"},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{"a", 1},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfEqualFold(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfEqualFold() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfEqualFold() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpfReplace(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"Lorem ipsum dolor sit amet",
				"ipsum",
				"muspi",
				-1.0,
			},
		},
		want: "Lorem muspi dolor sit amet",
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				"ipsum",
				"muspi",
				-1.0,
			},
		},
		want: "Lorem muspi muspi muspi dolor sit amet",
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				"ipsum",
				"muspi",
				1.0,
			},
		},
		want: "Lorem muspi ipsum ipsum dolor sit amet",
	}, {
		args: args{
			arguments: []interface{}{
				1.0,
				"ipsum",
				"muspi",
				1.0,
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				1.0,
				"muspi",
				1.0,
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				"ipsum",
				false,
				1.0,
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				"ipsum",
				"muspi",
				true,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfReplace(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfReplace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfReplace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpfReplaceAll(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"Lorem ipsum dolor sit amet",
				"ipsum",
				"muspi",
			},
		},
		want: "Lorem muspi dolor sit amet",
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				"ipsum",
				"muspi",
			},
		},
		want: "Lorem muspi muspi muspi dolor sit amet",
	}, {
		args: args{
			arguments: []interface{}{
				1.0,
				"ipsum",
				"muspi",
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				1.0,
				"muspi",
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				"Lorem ipsum ipsum ipsum dolor sit amet",
				"ipsum",
				false,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfReplaceAll(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfReplaceAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfReplaceAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpfToUpper(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"abc",
			},
		},
		want: "ABC",
	}, {
		args: args{
			arguments: []interface{}{
				"123",
			},
		},
		want: "123",
	}, {
		args: args{
			arguments: []interface{}{
				"a#%&123Bc",
			},
		},
		want: "A#%&123BC",
	}, {
		args: args{
			arguments: []interface{}{
				32.0,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfToUpper(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfToUpper() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfToUpper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_jpfToLower(t *testing.T) {
	type args struct {
		arguments []interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				"ABC",
			},
		},
		want: "abc",
	}, {
		args: args{
			arguments: []interface{}{
				"123",
			},
		},
		want: "123",
	}, {
		args: args{
			arguments: []interface{}{
				"A#%&123BC",
			},
		},
		want: "a#%&123bc",
	}, {
		args: args{
			arguments: []interface{}{
				32.0,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jpfToLower(tt.args.arguments)
			if (err != nil) != tt.wantErr {
				t.Errorf("jpfToLower() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jpfToLower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ImageNormalize(t *testing.T) {
	testCases := []struct {
		jmesPath       string
		expectedResult string
		wantErr        bool
	}{
		{
			jmesPath:       "image_normalize('nginx')",
			expectedResult: "docker.io/nginx:latest",
		},
		{
			jmesPath:       "image_normalize('docker.io/nginx')",
			expectedResult: "docker.io/nginx:latest",
		},
		{
			jmesPath:       "image_normalize('docker.io/library/nginx')",
			expectedResult: "docker.io/library/nginx:latest",
		},
		{
			jmesPath:       "image_normalize('ghcr.io/library/nginx')",
			expectedResult: "ghcr.io/library/nginx:latest",
		},
		{
			jmesPath:       "image_normalize('ghcr.io/nginx')",
			expectedResult: "ghcr.io/nginx:latest",
		},
		{
			jmesPath:       "image_normalize('ghcr.io/nginx:latest')",
			expectedResult: "ghcr.io/nginx:latest",
		},
		{
			jmesPath:       "image_normalize('ghcr.io/nginx:1.2')",
			expectedResult: "ghcr.io/nginx:1.2",
		},
		{
			jmesPath: "image_normalize('')",
			wantErr:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.jmesPath, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.jmesPath)
			assert.NilError(t, err)
			result, err := jp.Search("")
			if tc.wantErr {
				assert.Error(t, err, "JMESPath function 'image_normalize': bad image: docker.io/, defaultRegistry: docker.io, enableDefaultRegistryMutation: true: invalid reference format")
			} else {
				assert.NilError(t, err)
				res, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, res, tc.expectedResult)
			}
		})
	}
}
