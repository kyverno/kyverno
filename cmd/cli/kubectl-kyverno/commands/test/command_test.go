package test

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCommandWithInvalidArg(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: requires at least 1 arg(s), only received 0`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandWithInvalidFlag(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.SetArgs([]string{"--xxx"})
	err := cmd.Execute()
	assert.Error(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	expected := `Error: unknown flag: --xxx`
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(out)))
}

func TestCommandHelp(t *testing.T) {
	cmd := Command()
	assert.NotNil(t, cmd)
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	out, err := io.ReadAll(b)
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(out), cmd.Long))
}
func TestConvertNumericValuesToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "int to float64",
			input:    10,
			expected: 10.0,
		},
		{
			name:     "int32 to float64",
			input:    int32(20),
			expected: 20.0,
		},
		{
			name:     "int64 to float64",
			input:    int64(30),
			expected: 30.0,
		},
		{
			name:     "map with mixed values",
			input:    map[string]interface{}{"a": 5, "b": "string", "c": int32(10)},
			expected: map[string]interface{}{"a": 5.0, "b": "string", "c": 10.0},
		},
		{
			name:     "nested map",
			input:    map[string]interface{}{"a": map[string]interface{}{"b": int64(40)}},
			expected: map[string]interface{}{"a": map[string]interface{}{"b": 40.0}},
		},
		{
			name:     "slice of ints",
			input:    []interface{}{1, int32(2), int64(3), "text"},
			expected: []interface{}{1.0, 2.0, 3.0, "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertNumericValuesToFloat64(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestProcessResources(t *testing.T) {
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
	}

	expected := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3.0,
				},
			},
		},
	}

	processed := ProcessResources(resources)

	if !reflect.DeepEqual(processed, expected) {
		t.Errorf("Expected %v, got %v", expected, processed)
	}
}
