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
			resources: map[string]map[string]map[string]*unstructured.Unstructured{},
		},
		cp,
	)
}

func TestFakeContextProvider_GetGlobalReference(t *testing.T) {
	cp := &FakeContextProvider{}
	assert.Panics(t, func() { cp.GetGlobalReference("foo", "bar") })
}

func TestFakeContextProvider_GetImageData(t *testing.T) {
	cp := &FakeContextProvider{}
	assert.Panics(t, func() { cp.GetImageData("foo") })
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
		got, err := cp.ListResources("v1", "configmaps", "test-ns")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "configmaps", "wrong-ns")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "wrongs", "test-ns")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
	{
		got, err := cp.ListResources("wrong", "configmaps", "test-ns")
		assert.Error(t, err)
		assert.Nil(t, got)
	}
}
