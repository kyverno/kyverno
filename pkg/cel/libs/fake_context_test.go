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
		// empty namespace must return resources across all namespaces
		got, err := cp.ListResources("v1", "configmaps", "", nil)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(got.Items))
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

func TestFakeContextProvider_ListResources_AllNamespaces(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	makeConfigMap := func(ns, name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		}
	}

	cp := NewFakeContextProvider()
	assert.NoError(t, cp.AddResource(gvr, makeConfigMap("ns-a", "cm-1")))
	assert.NoError(t, cp.AddResource(gvr, makeConfigMap("ns-a", "cm-2")))
	assert.NoError(t, cp.AddResource(gvr, makeConfigMap("ns-b", "cm-3")))

	// scoped list still works
	got, err := cp.ListResources("v1", "configmaps", "ns-a", nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got.Items))

	// empty namespace = all namespaces
	got, err = cp.ListResources("v1", "configmaps", "", nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(got.Items))
}
