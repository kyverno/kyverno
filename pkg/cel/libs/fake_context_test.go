package libs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNewFakeContextProvider(t *testing.T) {
	cp := NewFakeContextProvider()
	assert.Equal(
		t, &FakeContextProvider{
			resources:        map[string]map[string]map[string]*unstructured.Unstructured{},
			images:           map[string]map[string]any{},
			globalReferences: map[string]any{},
		},
		cp,
	)
}

func TestFakeContextProvider_GetGlobalReference(t *testing.T) {
	cp := &FakeContextProvider{}
	v, err := cp.GetGlobalReference("foo", "bar")
	assert.NoError(t, err)
	assert.Nil(t, v)

	cp2 := NewFakeContextProvider()
	v, err = cp2.GetGlobalReference("missing", "")
	assert.NoError(t, err)
	assert.Nil(t, v)

	cp2.AddGlobalReference("key", map[string]any{"hello": "world"})
	val, err := cp2.GetGlobalReference("key", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"hello": "world"}, val)

	// Projection lookup: returns only the requested projection value
	cp3 := NewFakeContextProvider()
	cp3.AddGlobalReference("entry", map[string]any{
		"names": []interface{}{"dep-1", "dep-2"},
		"count": float64(2),
	})
	projVal, err := cp3.GetGlobalReference("entry", "names")
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"dep-1", "dep-2"}, projVal)

	projVal2, err := cp3.GetGlobalReference("entry", "count")
	assert.NoError(t, err)
	assert.Equal(t, float64(2), projVal2)

	// Missing projection returns error
	_, err = cp3.GetGlobalReference("entry", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "projection")

	// Projection on non-map data returns error
	cp4 := NewFakeContextProvider()
	cp4.AddGlobalReference("list-entry", []interface{}{"a", "b"})
	_, err = cp4.GetGlobalReference("list-entry", "proj")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an object")
}

func TestFakeContextProvider_AddGlobalReference_initializesNilMap(t *testing.T) {
	cp := &FakeContextProvider{}
	cp.AddGlobalReference("k", map[string]any{"v": true})
	got, err := cp.GetGlobalReference("k", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"v": true}, got)
}

func TestFakeContextProvider_AddImageData(t *testing.T) {
	cp := NewFakeContextProvider()

	imageName := "nginx:latest"
	imageData := map[string]any{
		"image":    "nginx:latest",
		"registry": "index.docker.io",
		"digest":   "sha256:12345",
		"manifest": map[string]any{
			"annotations": map[string]any{
				"org.opencontainers.image.base.name": "debian:latest",
			},
		},
	}
	cp.AddImageData(imageName, imageData)
	got, err := cp.GetImageData(imageName)
	assert.NoError(t, err)
	assert.Equal(t, imageData, got)
	got, err = cp.GetImageData("missing:latest")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestFakeContextProvider_PostResource(t *testing.T) {
	cp := &FakeContextProvider{}
	assert.Panics(t, func() { cp.PostResource("v1", "configmaps", "default", nil) })
}

func TestFakeContextProvider_AddResource(t *testing.T) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test-name",
		},
	}
	want, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cm)
	assert.NoError(t, err)
	cp := NewFakeContextProvider()
	assert.NoError(
		t,
		cp.AddResource(
			schema.GroupVersionResource{
				Version:  "v1",
				Resource: "configmaps",
			},
			cm,
		),
	)
	// get
	{
		got, err := cp.GetResource("v1", "configmaps", "test-ns", "test-name")
		assert.NoError(t, err)
		assert.Equal(t, want, got.Object)
	}
	{
		got, err := cp.GetResource("v1", "configmaps", "test-ns", "wrong-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.GetResource("v1", "configmaps", "wrong-ns", "test-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.GetResource("v1", "wrongs", "test-ns", "test-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.GetResource("wrong", "configmaps", "test-ns", "test-name")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	// list
	{
		got, err := cp.ListResources("v1", "configmaps", "test-ns", nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "configmaps", "wrong-ns", nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "wrongs", "test-ns", nil)
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.ListResources("wrong", "configmaps", "test-ns", nil)
		assert.Error(t, err)
		assert.Nil(t, got)
	}
}

func TestFakeContextProvider_Clone(t *testing.T) {
	original := &FakeContextProvider{
		generatedResources: make([]*unstructured.Unstructured, 0),
	}
	original.SetGenerateContext("policyA", "default", "triggerA", "default", "v1", "", "Pod", "123", true)

	// Pre-populate original to verify Clone() initializes an isolated, empty slice
	originalResource := &unstructured.Unstructured{Object: map[string]interface{}{"kind": "ConfigMap"}}
	original.generatedResources = append(original.generatedResources, originalResource)

	cloneContext := original.Clone()
	clone, ok := cloneContext.(*FakeContextProvider)
	assert.True(t, ok, "cloned context should be of type *FakeContextProvider")

	clone.SetGenerateContext("policyB", "kube-system", "triggerB", "kube-system", "v1", "", "Service", "456", false)

	assert.Equal(t, "policyA", original.policyName)
	assert.Equal(t, true, original.restoreCache)

	assert.Equal(t, "policyB", clone.policyName)
	assert.Equal(t, false, clone.restoreCache)

	// Verify slice isolation
	assert.Len(t, original.generatedResources, 1)
	assert.Len(t, clone.generatedResources, 0)

	// Verify mutations don't leak to the original provider
	mockResource := &unstructured.Unstructured{Object: map[string]interface{}{"kind": "Pod"}}
	clone.generatedResources = append(clone.generatedResources, mockResource)

	assert.Len(t, original.generatedResources, 1)
	assert.Len(t, clone.generatedResources, 1)
}
