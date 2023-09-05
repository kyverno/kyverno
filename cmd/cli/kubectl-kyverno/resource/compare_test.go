package resource

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
