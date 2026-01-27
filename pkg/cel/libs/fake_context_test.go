package libs

import (
	"io"
	"net/http"
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
	assert.NotNil(t, cp)
	assert.NotNil(t, cp.resources)
	assert.NotNil(t, cp.images)
	assert.NotNil(t, cp.globalContext)
	assert.NotNil(t, cp.HTTPClient())
}

func TestFakeContextProvider_GetGlobalReference(t *testing.T) {
	cp := NewFakeContextProvider()
	got, err := cp.GetGlobalReference("missing", "")
	assert.NoError(t, err)
	assert.Nil(t, got)

	cp.AddGlobalReference("foo", "", map[string]any{"hello": "world"})
	got, err = cp.GetGlobalReference("foo", "")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"hello": "world"}, got)

	// projection fallback to default
	got, err = cp.GetGlobalReference("foo", "projection")
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"hello": "world"}, got)

	// missing projection (no default) returns error with available projections
	cp.AddGlobalReference("bar", "proj1", map[string]any{"a": 1})
	got, err = cp.GetGlobalReference("bar", "proj2")
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "projection \"proj2\" not found")
	assert.Contains(t, err.Error(), "available projections")
	assert.Contains(t, err.Error(), "context file")
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
	cp := NewFakeContextProvider()
	created, err := cp.PostResource("v1", "configmaps", "default", map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name": "cm-1",
		},
		"data": map[string]any{
			"key": "value",
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, created)

	got, err := cp.GetResource("v1", "configmaps", "default", "cm-1")
	assert.NoError(t, err)
	assert.Equal(t, "cm-1", got.GetName())
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
			Labels: map[string]string{
				"app": "test",
			},
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
		got, err := cp.ListResources("v1", "configmaps", "test-ns", map[string]string{"app": "test"})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(got.Items))
	}
	{
		got, err := cp.ListResources("v1", "configmaps", "test-ns", map[string]string{"app": "nope"})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(got.Items))
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

func TestFakeContextProvider_ToGVR_Fallback(t *testing.T) {
	cp := NewFakeContextProvider()
	gvr, err := cp.ToGVR("v1", "ConfigMap")
	assert.NoError(t, err)
	assert.Equal(t, "configmaps", gvr.Resource)
}

func TestFakeContextProvider_HTTPStub(t *testing.T) {
	cp := NewFakeContextProvider()
	cp.AddHTTPStub("GET", "http://example.test", 200, nil, []byte(`{"body":"ok"}`))

	req, err := http.NewRequest("GET", "http://example.test", nil)
	assert.NoError(t, err)
	resp, err := cp.HTTPClient().Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"body":"ok"}`, string(b))
}

func TestFakeContextProvider_HTTPStub_MissingStubError(t *testing.T) {
	cp := NewFakeContextProvider()
	cp.AddHTTPStub("GET", "http://example.test", 200, nil, []byte("{}"))
	req, err := http.NewRequest("GET", "http://other.test", nil)
	assert.NoError(t, err)
	_, err = cp.HTTPClient().Do(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no HTTP stub found for GET http://other.test")
	assert.Contains(t, err.Error(), "available stubs")
	assert.Contains(t, err.Error(), "context file")
}

func TestFakeContextProvider_GetImageData_MissingError(t *testing.T) {
	cp := NewFakeContextProvider()
	cp.AddImageData("nginx:1.21", map[string]any{"image": "nginx:1.21"})
	_, err := cp.GetImageData("missing:latest")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image data for missing:latest not found")
	assert.Contains(t, err.Error(), "available images")
	assert.Contains(t, err.Error(), "context file")
}
