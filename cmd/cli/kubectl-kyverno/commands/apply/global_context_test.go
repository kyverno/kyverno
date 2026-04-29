package apply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestExtractGlobalContextEntries(t *testing.T) {
	gce := &unstructured.Unstructured{}
	gce.SetAPIVersion("kyverno.io/v2")
	gce.SetKind("GlobalContextEntry")
	gce.SetName("my-cache")
	gce.Object["spec"] = map[string]interface{}{
		"kubernetesResource": map[string]interface{}{
			"version":  "v1",
			"resource": "configmaps",
		},
	}

	cm := makeUnstructured("v1", "ConfigMap", "default", "my-cm")
	deploy := makeUnstructured("apps/v1", "Deployment", "default", "my-deploy")

	resources := []*unstructured.Unstructured{gce, cm, deploy}
	regular, entries, err := extractGlobalContextEntries(resources)
	require.NoError(t, err)

	assert.Len(t, entries, 1)
	assert.Equal(t, "my-cache", entries[0].Name)
	assert.Len(t, regular, 2)
}

func TestExtractGlobalContextEntries_NoGCE(t *testing.T) {
	cm := makeUnstructured("v1", "ConfigMap", "default", "my-cm")
	resources := []*unstructured.Unstructured{cm}

	regular, entries, err := extractGlobalContextEntries(resources)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
	assert.Len(t, regular, 1)
}

func TestExtractGlobalContextEntries_Empty(t *testing.T) {
	regular, entries, err := extractGlobalContextEntries(nil)
	require.NoError(t, err)
	assert.Len(t, entries, 0)
	assert.Len(t, regular, 0)
}

func TestStaticEntry_GetDefault(t *testing.T) {
	objects := []interface{}{
		map[string]interface{}{"metadata": map[string]interface{}{"name": "item1"}},
		map[string]interface{}{"metadata": map[string]interface{}{"name": "item2"}},
	}
	entry := &staticEntry{objects: objects, projections: map[string]interface{}{}}

	result, err := entry.Get("")
	require.NoError(t, err)
	items, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, items, 2)
}

func TestStaticEntry_GetProjection(t *testing.T) {
	entry := &staticEntry{
		objects: nil,
		projections: map[string]interface{}{
			"names": []string{"a", "b"},
		},
	}

	result, err := entry.Get("names")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestStaticEntry_GetProjectionNotFound(t *testing.T) {
	entry := &staticEntry{objects: nil, projections: map[string]interface{}{}}

	_, err := entry.Get("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "projection")
}
