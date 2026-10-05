package payload

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDocument(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`{"id":9007199254740993}`, `[1,"a",null]`, `"hello"`, `true`, `9007199254740993`, `null`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			document, err := ParseDocument("input.json", []byte(input))
			require.NoError(t, err)
			require.Equal(t, input, string(document.Raw))
		})
	}
	for _, input := range []string{``, `{"broken":`, `null null`, `NaN`} {
		t.Run("invalid-"+input, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDocument("input.json", []byte(input))
			require.Error(t, err)
		})
	}
	document, err := ParseDocument("input.yaml", []byte("id: 9007199254740993\n"))
	require.NoError(t, err)
	require.JSONEq(t, `{"id":9007199254740993}`, string(document.Raw))
}

func TestParseDocumentRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		file    string
		content string
		err     string
	}{
		"multi-document yaml": {file: "input.yaml", content: "a: 1\n---\nb: 2\n", err: "exactly one YAML document"},
		"empty yaml":          {file: "input.yml", content: ""},
		"unknown extension":   {file: "input.txt", content: `{"a":1}`, err: "unrecognized payload format"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseDocument(tc.file, []byte(tc.content))
			require.Error(t, err)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestParseDocumentYAMLScalarRoots(t *testing.T) {
	t.Parallel()
	for input, expected := range map[string]string{
		"- 1\n- a\n": `[1,"a"]`,
		"hello\n":    `"hello"`,
		"true\n":     `true`,
		"~\n":        `null`,
	} {
		document, err := ParseDocument("input.yaml", []byte(input))
		require.NoError(t, err)
		require.JSONEq(t, expected, string(document.Raw))
	}
}

func TestDocumentObjectRequiresObjectRoot(t *testing.T) {
	t.Parallel()
	object, err := (&Document{Name: "obj.json", Raw: []byte(`{"a":1}`)}).Object()
	require.NoError(t, err)
	require.EqualValues(t, 1, object.Object["a"])
	for _, raw := range []string{`[1]`, `"s"`, `1`, `true`, `null`} {
		_, err := (&Document{Name: "doc.json", Raw: []byte(raw)}).Object()
		require.ErrorContains(t, err, "requires an object root", raw)
	}
	_, err = Objects([]*Document{{Name: "a.json", Raw: []byte(`{}`)}, {Name: "b.json", Raw: []byte(`[]`)}})
	require.ErrorContains(t, err, "b.json")
}
