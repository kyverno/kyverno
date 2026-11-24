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
		op, object, oldObject := AdmissionRequestShape("", resource, nil)
		assert.Equal(t, admissionv1.Create, op)
		assert.Equal(t, resource, object)
		assert.Nil(t, oldObject)
	})
	t.Run("CREATE", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("CREATE", resource, nil)
		assert.Equal(t, admissionv1.Create, op)
		assert.Equal(t, resource, object)
		assert.Nil(t, oldObject)
	})
	t.Run("CREATE ignores oldObject override", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("CREATE", resource, map[string]interface{}{"spec": map[string]interface{}{"x": 1}})
		assert.Equal(t, admissionv1.Create, op)
		assert.Equal(t, resource, object)
		assert.Nil(t, oldObject)
	})
	t.Run("UPDATE sets object and oldObject", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("UPDATE", resource, nil)
		assert.Equal(t, admissionv1.Update, op)
		assert.Equal(t, resource, object)
		assert.Equal(t, resource.Object, oldObject.(*unstructured.Unstructured).Object)
	})
	t.Run("DELETE sets only oldObject", func(t *testing.T) {
		op, object, oldObject := AdmissionRequestShape("DELETE", resource, nil)
		assert.Equal(t, admissionv1.Delete, op)
		assert.Nil(t, object)
		assert.Equal(t, resource.Object, oldObject.(*unstructured.Unstructured).Object)
	})
	t.Run("UPDATE merges a partial oldObject override onto the resource", func(t *testing.T) {
		override := map[string]interface{}{
			"spec": map[string]interface{}{"host": "old.example.com"},
		}
		_, _, oldObject := AdmissionRequestShape("UPDATE", resource, override)
		got := oldObject.(*unstructured.Unstructured)
		assert.Equal(t, "old.example.com", got.Object["spec"].(map[string]interface{})["host"])
		// unrelated fields are preserved from the resource
		assert.Equal(t, "test-pod", got.Object["metadata"].(map[string]interface{})["name"])
		// the override must not mutate the input resource
		_, hasSpec := resource.Object["spec"]
		assert.False(t, hasSpec, "override must not mutate the input resource")
	})
	t.Run("DELETE merges a partial oldObject override onto the resource", func(t *testing.T) {
		override := map[string]interface{}{
			"metadata": map[string]interface{}{"name": "deleted-pod"},
		}
		_, _, oldObject := AdmissionRequestShape("DELETE", resource, override)
		got := oldObject.(*unstructured.Unstructured)
		assert.Equal(t, "deleted-pod", got.Object["metadata"].(map[string]interface{})["name"])
	})
	t.Run("null override value deletes the leaf key so CEL has() sees it as absent", func(t *testing.T) {
		resourceWithAnnotation := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":        "test-pod",
				"annotations": map[string]interface{}{"example.com/paused": "true"},
			},
		}}
		override := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{"example.com/paused": nil},
			},
		}
		_, _, oldObject := AdmissionRequestShape("UPDATE", resourceWithAnnotation, override)
		got := oldObject.(*unstructured.Unstructured)
		annotations := got.Object["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
		_, ok := annotations["example.com/paused"]
		assert.False(t, ok, "an explicit null override deletes the key instead of storing null")
	})
	t.Run("null override on an entire subtree removes it", func(t *testing.T) {
		resourceWithSpec := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]interface{}{"name": "test-pod"},
			"spec": map[string]interface{}{
				"config": map[string]interface{}{"resourceName": "xyz"},
			},
		}}
		override := map[string]interface{}{
			"spec": map[string]interface{}{"config": nil},
		}
		_, _, oldObject := AdmissionRequestShape("UPDATE", resourceWithSpec, override)
		got := oldObject.(*unstructured.Unstructured)
		_, ok := got.Object["spec"].(map[string]interface{})["config"]
		assert.False(t, ok, "no spec.config")
	})
}

func TestMergeJSON(t *testing.T) {
	t.Run("nested maps merge recursively", func(t *testing.T) {
		got, err := mergeJSON(
			map[string]interface{}{"a": map[string]interface{}{"x": int64(1), "y": int64(2)}},
			map[string]interface{}{"a": map[string]interface{}{"y": int64(3), "z": int64(4)}},
		)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"x": int64(1), "y": int64(3), "z": int64(4)}, got["a"])
	})
	t.Run("non-map override value replaces the target leaf", func(t *testing.T) {
		got, err := mergeJSON(map[string]interface{}{"a": "old", "b": "keep"}, map[string]interface{}{"a": "new"})
		assert.NoError(t, err)
		assert.Equal(t, "new", got["a"])
		assert.Equal(t, "keep", got["b"])
	})
	t.Run("explicit null deletes the target key instead of storing nil", func(t *testing.T) {
		got, err := mergeJSON(map[string]interface{}{"a": "old", "b": "keep"}, map[string]interface{}{"a": nil})
		assert.NoError(t, err)
		_, ok := got["a"]
		assert.False(t, ok)
		assert.Equal(t, "keep", got["b"])
	})
	t.Run("null nested under a key absent from the target is not stored", func(t *testing.T) {
		// A Deployment with no annotations at all, and an override that
		// sets one annotation and nulls another. The null must not survive as a
		// nested JSON null, or CEL's has()/`in` would see it as present.
		got, err := mergeJSON(
			map[string]interface{}{"metadata": map[string]interface{}{"name": "d"}},
			map[string]interface{}{"metadata": map[string]interface{}{"annotations": map[string]interface{}{"foo": nil, "bar": "x"}}},
		)
		assert.NoError(t, err)
		annotations := got["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
		_, hasFoo := annotations["foo"]
		assert.False(t, hasFoo, "a null nested under a map the target doesn't have must not be stored")
		assert.Equal(t, "x", annotations["bar"])
	})
	t.Run("override map replaces a non-map target value wholesale", func(t *testing.T) {
		got, err := mergeJSON(map[string]interface{}{"a": "scalar"}, map[string]interface{}{"a": map[string]interface{}{"x": int64(1)}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"x": int64(1)}, got["a"])
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
	_, object, oldObject := AdmissionRequestShape("UPDATE", resource, nil)
	object.(*unstructured.Unstructured).SetLabels(map[string]string{"mutated": "true"})
	assert.Empty(t, oldObject.(*unstructured.Unstructured).GetLabels())
}

func TestMergeOldObjectFallback(t *testing.T) {
	resource := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{"name": "p1", "labels": map[string]interface{}{"tier": "new"}},
	}}
	// A func can't be marshaled, so the merge fails and the override is ignored.
	got := mergeOldObject(resource, map[string]interface{}{"bad": func() {}})
	assert.Equal(t, resource.Object, got.Object)
	got.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["tier"] = "changed"
	assert.Equal(t, "new", resource.Object["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["tier"], "the fallback is a copy")

	_, err := mergeJSON(map[string]interface{}{"bad": func() {}}, map[string]interface{}{})
	assert.Error(t, err)
	_, err = mergeJSON(map[string]interface{}{}, map[string]interface{}{"bad": func() {}})
	assert.Error(t, err)
}
