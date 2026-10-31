package apply

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func newConfigMapGCE(name string, projections []kyvernov2.GlobalContextEntryProjection) *kyvernov2.GlobalContextEntry {
	return &kyvernov2.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kyvernov2.GlobalContextEntrySpec{
			KubernetesResource: &kyvernov2.KubernetesResource{
				Version:  "v1",
				Resource: "configmaps",
			},
			Projections: projections,
		},
	}
}

func fakeConfigMapClient(t *testing.T) dclient.Interface {
	cm := makeUnstructured("v1", "ConfigMap", "default", "my-cm")
	client, err := dclient.NewFakeClient(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
		cm,
	)
	require.NoError(t, err)
	return client
}

func TestBuildGlobalContextStore_ExpressionOnly_NoJMESPathEngineRequired(t *testing.T) {
	gce := newConfigMapGCE("my-cache", []kyvernov2.GlobalContextEntryProjection{
		{Name: "count", Expression: "object.size()"},
	})

	s, err := buildGlobalContextStore(context.Background(), []*kyvernov2.GlobalContextEntry{gce}, fakeConfigMapClient(t), nil)
	require.NoError(t, err)

	entry, ok := s.Get("my-cache")
	require.True(t, ok)
	result, err := entry.Get("count")
	require.NoError(t, err)
	assert.Equal(t, int64(1), result)
}

func TestBuildGlobalContextStore_JMESPathProjectionRequiresEngine(t *testing.T) {
	gce := newConfigMapGCE("my-cache", []kyvernov2.GlobalContextEntryProjection{
		{Name: "count", JMESPath: "length(@)"},
	})

	_, err := buildGlobalContextStore(context.Background(), []*kyvernov2.GlobalContextEntry{gce}, fakeConfigMapClient(t), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JMESPath engine is required")
}
