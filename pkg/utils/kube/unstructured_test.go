package kube

import (
	"fmt"
	"reflect"
	"testing"
	"time"

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
			name: "preserve integers when preparing an unstructured object for a write",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": []interface{}{int64(9007199254740993), float64(2), 1.5, "9007199254740993"},
			}},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": []interface{}{int64(9007199254740993), int64(2), 1.5, "9007199254740993"},
			}},
		},
		{
			name: "normalize static YAML values alongside exact CEL integers",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
					"values":   []interface{}{80, int64(9007199254740993), 1.5},
					"created":  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
				},
			}},
			want: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": int64(3),
					"values":   []interface{}{int64(80), int64(9007199254740993), 1.5},
					"created":  "2026-01-02T03:04:05Z",
				},
			}},
		},
		{
			name: "preserve integers in plain maps",
			obj:  map[string]interface{}{"revision": int64(9007199254740993)},
			want: &unstructured.Unstructured{Object: map[string]interface{}{"revision": int64(9007199254740993)}},
		},
		{
			name: "preserve integers in typed objects",
			obj:  struct{ Revision int64 }{Revision: 9007199254740993},
			want: &unstructured.Unstructured{Object: map[string]interface{}{"Revision": int64(9007199254740993)}},
		},
		{
			name: "Test invalid JSON",
			obj:  func() {},
			err:  fmt.Errorf("json: unsupported type: func()"),
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
			err:  fmt.Errorf("json: cannot unmarshal array into Go value of type map[string]interface {}"),
		},

		{
			name: "Test number input",
			obj:  123,
			err:  fmt.Errorf("json: cannot unmarshal number into Go value of type map[string]interface {}"),
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

// TestObjToUnstructuredDoesNotAliasInput verifies that conversion produces an independent object tree.
func TestObjToUnstructuredDoesNotAliasInput(t *testing.T) {
	original := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{"revision": int64(9007199254740993), "ratio": float64(2), "replicas": 3},
	}}
	got, err := ObjToUnstructured(original)
	if err != nil {
		t.Fatal(err)
	}
	got.Object["spec"].(map[string]interface{})["revision"] = int64(1)
	if gotRevision := original.Object["spec"].(map[string]interface{})["revision"]; gotRevision != int64(9007199254740993) {
		t.Fatalf("conversion mutated original revision: %v", gotRevision)
	}
}
