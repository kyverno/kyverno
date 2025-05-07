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

var jmespathInterface = newImplementation(config.NewDefaultConfiguration(false))

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
			jp, err := jmespathInterface.Query(tc.jmesPath)
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
			jp, err := jmespathInterface.Query(fmt.Sprintf(`to_string(parse_json('%s'))`, tc))
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
			jp, err := jmespathInterface.Query(tc.input)
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
			jp, err := jmespathInterface.Query(fmt.Sprintf(`parse_yaml('%s')`, tc.input))
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
			jp, err := jmespathInterface.Query(tc.jmesPath)
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
	// TODO: fix this in https://github.com/jmespath-community/go-jmespath
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
			jp, err := jmespathInterface.Query(tc.jmesPath)
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
	jp, err := jmespathInterface.Query("replace_all('Lorem ipsum dolor sit amet', 'ipsum', 'muspi')")
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
			jp, err := jmespathInterface.Query(tc.jmesPath)
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
			jp, err := jmespathInterface.Query(tc.jmesPath)
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
	jp, err := jmespathInterface.Query("trim('¡¡¡Hello, Gophers!!!', '!¡')")
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
	jp, err := jmespathInterface.Query("split('Hello, Gophers', ', ')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.([]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, split[0], "Hello")
	assert.Equal(t, split[1], "Gophers")
}

func Test_HasPrefix(t *testing.T) {
	jp, err := jmespathInterface.Query("starts_with('Gophers', 'Go')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	split, ok := result.(bool)
	assert.Assert(t, ok)
	assert.Equal(t, split, true)
}

func Test_HasSuffix(t *testing.T) {
	jp, err := jmespathInterface.Query("ends_with('Amigo', 'go')")
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

	query, err := jmespathInterface.Query("regex_match('12.*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_RegexMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := jmespathInterface.Query("regex_match('12.*', abs(foo))")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_PatternMatch(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = "prefix-foo"

	query, err := jmespathInterface.Query("pattern_match('prefix-*', foo)")
	assert.NilError(t, err)

	result, err := query.Search(data)
	assert.NilError(t, err)
	assert.Equal(t, true, result)
}

func Test_PatternMatchWithNumber(t *testing.T) {
	data := make(map[string]interface{})
	data["foo"] = -12.0

	query, err := jmespathInterface.Query("pattern_match('12*', abs(foo))")
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
	query, err := jmespathInterface.Query(`regex_replace_all('([Hh]e|G)l', spec.field, '${2}G')`)
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

	query, err := jmespathInterface.Query(`regex_replace_all_literal('[Hh]el?', spec.field, 'G')`)
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

			query, err := jmespathInterface.Query("label_match(`" + tc.test + "`, metadata.labels)")
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
	jp, err := jmespathInterface.Query("base64_decode('SGVsbG8sIHdvcmxkIQ==')")
	assert.NilError(t, err)

	result, err := jp.Search("")
	assert.NilError(t, err)

	str, ok := result.(string)
	assert.Assert(t, ok)
	assert.Equal(t, str, "Hello, world!")
}

func Test_Base64Encode(t *testing.T) {
	jp, err := jmespathInterface.Query("base64_encode('Hello, world!')")
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

	query, err := jmespathInterface.Query(`base64_decode(data.example1)`)
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
			jp, err := jmespathInterface.Query(tc.jmesPath)
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
	// TODO: fix this in https://github.com/jmespath-community/go-jmespath
