package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNormalizeOperation(t *testing.T) {
	tests := []struct {
		operation string
		want      string
		wantErr   bool
	}{
		{operation: "", want: ""},
		{operation: "CREATE", want: "CREATE"},
		{operation: "UPDATE", want: "UPDATE"},
		{operation: "DELETE", want: "DELETE"},
		{operation: "CONNECT", wantErr: true},
		{operation: "delete", wantErr: true},
		{operation: "foo", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			got, err := NormalizeOperation(tt.operation)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestNormalizeValuesOperation(t *testing.T) {
	tests := []struct {
		operation string
		wantErr   bool
	}{
		{operation: ""},
		{operation: "CREATE"},
		{operation: "UPDATE"},
		{operation: "DELETE"},
		{operation: "CONNECT"},
		{operation: "connect", wantErr: true},
		{operation: "foo", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			got, err := NormalizeValuesOperation(tt.operation)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.operation, got)
			}
		})
	}
}

func TestAdmissionRequestShape(t *testing.T) {
	resource := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "test-pod"},
	}}
	t.Run("default is CREATE", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("", resource)
		assert.Equal(t, admissionv1.Create, op)
		assert.Equal(t, resource, object)
		assert.Nil(t, oldObject)
	})
	t.Run("CREATE", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("CREATE", resource)
		assert.Equal(t, admissionv1.Create, op)
		assert.Equal(t, resource, object)
		assert.Nil(t, oldObject)
	})
	t.Run("UPDATE sets object and oldObject", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("UPDATE", resource)
		assert.Equal(t, admissionv1.Update, op)
		assert.Equal(t, resource, object)
		assert.Equal(t, resource.Object, oldObject.(*unstructured.Unstructured).Object)
	})
	t.Run("DELETE sets only oldObject", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("DELETE", resource)
		assert.Equal(t, admissionv1.Delete, op)
		assert.Nil(t, object)
		assert.Equal(t, resource.Object, oldObject.(*unstructured.Unstructured).Object)
	})
}

func TestResolveOperation(t *testing.T) {
	t.Run("explicit operation wins", func(t *testing.T) {
		p := &PolicyProcessor{Operation: "DELETE"}
		assert.Equal(t, "DELETE", p.resolveOperation())
	})
	t.Run("empty without variables", func(t *testing.T) {
		p := &PolicyProcessor{}
		assert.Equal(t, "", p.resolveOperation())
	})
}

func TestAdmissionRequestShapeUpdateDeepCopy(t *testing.T) {
	// the oldObject for UPDATE must be a copy so mutations of the object do not
	// leak into the old state
	resource := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "test-pod"},
	}}
	_, object, oldObject := AdmissionRequestShape("UPDATE", resource)
	object.(*unstructured.Unstructured).SetLabels(map[string]string{"mutated": "true"})
	assert.Empty(t, oldObject.(*unstructured.Unstructured).GetLabels())
}
