package kube

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConvertToUnstructured(t *testing.T) {
	type testCase struct {
		name string
		obj  interface{}
		want *unstructured.Unstructured
		err  error
	}
	testCases := []testCase{
		{
			name: "Test valid input",
			obj: map[string]interface{}{
				"key": "value",
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"key": "value",
			}},
		},
		{
			name: "Test invalid JSON",
			obj:  func() {},
			err:  fmt.Errorf("func() is unsupported type"),
		},
		{
			name: "Test struct input",
			obj: struct {
				Key string
			}{
				Key: "value",
			},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"Key": "value",
			}},
		},
		{
			name: "Test slice input",
			obj:  []string{"a", "b", "c"},
			err:  fmt.Errorf("ReadMapCB: expect { or n, but found [, error found in #1 byte of ...|[\"a\",\"b\",\"c|..., bigger context ...|[\"a\",\"b\",\"c\"]|..."),
		},

		{
			name: "Test number input",
			obj:  123,
			err:  fmt.Errorf("ReadMapCB: expect { or n, but found 1, error found in #1 byte of ...|123|..., bigger context ...|123|..."),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ObjToUnstructured(tc.obj)
			if (err != nil) != (tc.err != nil) {
				t.Errorf("expected error %v but got %v", tc.err, err)
			}
			if err != nil && tc.err != nil {
				if !reflect.DeepEqual(err.Error(), tc.err.Error()) {
					t.Errorf("expected error %v but got %v", tc.err, err)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected %v but got %v", tc.want, got)
			}
		})
	}
}

func TestBytesToUnstructured(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected *unstructured.Unstructured
	}{
		{
			name: "Test valid JSON",
			data: []byte(`{"key": "value"}`),
			expected: &unstructured.Unstructured{Object: map[string]interface{}{
				"key": "value",
			}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := BytesToUnstructured(tc.data)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected %v, but got %v", tc.expected, actual)
			}
		})
	}
}
