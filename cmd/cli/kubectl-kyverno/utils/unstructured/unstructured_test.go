package unstructured

import (
	"errors"
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
		e       unstructured.Unstructured
		tidy    bool
		want    bool
		wantErr bool
	}{{
		a:       unstructured.Unstructured{},
		e:       unstructured.Unstructured{},
		tidy:    true,
		want:    true,
		wantErr: false,
	}, {
		a:       unstructured.Unstructured{},
		e:       unstructured.Unstructured{},
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
		e: unstructured.Unstructured{
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
		e: unstructured.Unstructured{
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
		e: unstructured.Unstructured{
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
			got, err := Compare(tt.a, tt.e, tt.tidy)
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

func TestFixupGenerateLabels(t *testing.T) {
	tests := []struct {
		name string
		obj  unstructured.Unstructured
		want unstructured.Unstructured
	}{{
		name: "not set",
	}, {
		name: "empty",
		obj:  unstructured.Unstructured{Object: map[string]interface{}{}},
		want: unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "kyverno",
				},
			},
		}},
	}, {
		name: "with label",
		obj: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "kyverno",
					},
				},
			},
		},
		want: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "kyverno",
					},
				},
			},
		},
	}, {
		name: "with generate labels",
		obj: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"foo":                                   "bar",
						"generate.kyverno.io/policy-name":       "add-networkpolicy",
						"generate.kyverno.io/policy-namespace":  "",
						"generate.kyverno.io/rule-name":         "default-deny",
						"generate.kyverno.io/trigger-group":     "",
						"generate.kyverno.io/trigger-kind":      "Namespace",
						"generate.kyverno.io/trigger-name":      "hello-world-namespace",
						"generate.kyverno.io/trigger-namespace": "default",
						"generate.kyverno.io/trigger-version":   "v1",
					},
				},
			},
		},
		want: unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "kyverno",
						"foo":                          "bar",
					},
				},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FixupGenerateLabels(tt.obj)
			if !reflect.DeepEqual(tt.obj, tt.want) {
				t.Errorf("FixupGenerateLabels() = %v, want %v", tt.obj, tt.want)
			}
		})
	}
}

func Test_compare(t *testing.T) {
	errorMarshaller := func(count int) marshaler {
		var current = 0
		return func(obj *unstructured.Unstructured) ([]byte, error) {
			if current == count {
				return nil, errors.New("test")
			}
			current++
			return defaultMarshaler(obj)
		}
	}
	tests := []struct {
		name      string
		a         unstructured.Unstructured
		e         unstructured.Unstructured
		marshaler marshaler
		patcher   patcher
		want      bool
		wantErr   bool
	}{{
		name:      "nil marshaler",
		a:         unstructured.Unstructured{},
		e:         unstructured.Unstructured{},
		marshaler: nil,
		patcher:   defaultPatcher,
		want:      true,
		wantErr:   false,
	}, {
		name:      "nil patcher",
		a:         unstructured.Unstructured{},
		e:         unstructured.Unstructured{},
		marshaler: defaultMarshaler,
		patcher:   nil,
		want:      true,
		wantErr:   false,
	}, {
		name:      "error patcher",
		a:         unstructured.Unstructured{},
		e:         unstructured.Unstructured{},
		marshaler: defaultMarshaler,
		patcher: func(originalJSON, modifiedJSON []byte) ([]byte, error) {
			return nil, errors.New("test")
		},
		want:    false,
		wantErr: true,
	}, {
		name:      "error marshaller",
		a:         unstructured.Unstructured{},
		e:         unstructured.Unstructured{},
		marshaler: errorMarshaller(0),
		patcher:   defaultPatcher,
		want:      false,
		wantErr:   true,
	}, {
		name:      "error marshaller",
		a:         unstructured.Unstructured{},
		e:         unstructured.Unstructured{},
		marshaler: errorMarshaller(1),
		patcher:   defaultPatcher,
		want:      false,
		wantErr:   true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compare(tt.a, tt.e, tt.marshaler, tt.patcher)
			if (err != nil) != tt.wantErr {
				t.Errorf("compare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
