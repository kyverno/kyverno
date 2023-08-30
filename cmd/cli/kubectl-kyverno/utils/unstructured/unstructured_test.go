package unstructured

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestTidyObject(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want interface{}
	}{{
		obj:  "string",
		want: "string",
	}, {
		obj:  map[string]interface{}{},
		want: nil,
	}, {
		obj:  nil,
		want: nil,
	}, {
		obj:  []interface{}{},
		want: nil,
	}, {
		obj: map[string]interface{}{
			"map": nil,
		},
		want: nil,
	}, {
		obj: map[string]interface{}{
			"map": map[string]interface{}{},
		},
		want: nil,
	}, {
		obj: map[string]interface{}{
			"map": map[string]interface{}{
				"foo": "bar",
			},
		},
		want: map[string]interface{}{
			"map": map[string]interface{}{
				"foo": "bar",
			},
		},
	}, {
		obj:  []interface{}{[]interface{}{}},
		want: nil,
	}, {
		obj:  []interface{}{1},
		want: []interface{}{1},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TidyObject(tt.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TidyObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTidy(t *testing.T) {
	tests := []struct {
		name string
		obj  unstructured.Unstructured
		want unstructured.Unstructured
	}{{
		obj:  unstructured.Unstructured{},
		want: unstructured.Unstructured{},
	}, {
		obj: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		want: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Tidy(tt.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tidy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		a       unstructured.Unstructured
		b       unstructured.Unstructured
		tidy    bool
		want    bool
		wantErr bool
	}{{
		a:       unstructured.Unstructured{},
		b:       unstructured.Unstructured{},
		tidy:    true,
		want:    true,
		wantErr: false,
	}, {
		a:       unstructured.Unstructured{},
		b:       unstructured.Unstructured{},
		tidy:    false,
		want:    true,
		wantErr: false,
	}, {
		a: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		b: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		tidy:    true,
		want:    true,
		wantErr: false,
	}, {
		a: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
					"bar": map[string]interface{}{},
				},
			},
		},
		b: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		tidy:    true,
		want:    true,
		wantErr: false,
	}, {
		a: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
					"bar": map[string]interface{}{},
				},
			},
		},
		b: unstructured.Unstructured{
			Object: map[string]interface{}{
				"map": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		tidy:    false,
		want:    false,
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compare(tt.a, tt.b, tt.tidy)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
