package utils

import (
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestConvertToNative(t *testing.T) {
	tests := []struct {
		name    string
		value   ref.Val
		want    any
		wantErr bool
	}{{
		name:  "bool ok",
		value: types.False,
		want:  false,
	}, {
		name:    "string ko",
		value:   types.String("false"),
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToNative[bool](tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestConvertObjectToUnstructured(t *testing.T) {
	tests := []struct {
		name    string
		obj     any
		want    *unstructured.Unstructured
		wantErr bool
	}{{
		name: "nil",
		obj:  nil,
		want: &unstructured.Unstructured{},
	}, {
		name: "error",
		obj: map[string]string{
			"foo": "bar",
		},
		wantErr: true,
	}, {
		name: "ok",
		obj: &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		},
		want: &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name":              "foo",
					"creationTimestamp": nil,
				},
				"spec":   map[string]any{},
				"status": map[string]any{},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertObjectToUnstructured(tt.obj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestObjectToResolveVal(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Object
		want    any
		wantErr bool
	}{{
		name:    "nil",
		obj:     nil,
		want:    nil,
		wantErr: false,
	}, {
		name: "namespace",
		obj: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
		want: map[string]any{
			"metadata": map[string]any{
				"name":              "test",
				"creationTimestamp": nil,
			},
			"spec":   map[string]any{},
			"status": map[string]any{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ObjectToResolveVal(tt.obj)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		want    map[string]any
		wantErr bool
	}{{
		name: "nil",
	}, {
		name: "namespace",
		data: &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
		want: map[string]any{
			"metadata": map[string]any{
				"name":              "test",
				"creationTimestamp": nil,
			},
			"spec":   map[string]any{},
			"status": map[string]any{},
		},
	}, {
		name:    "error",
		data:    func() {},
		wantErr: true,
	},
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValue(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
