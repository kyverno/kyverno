package apply

import (
	"bytes"
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

// A projection setting both jmesPath and expression is rejected by the CRD, so the CLI must
// reject it too: otherwise the expression branch wins, jmesPath is silently discarded, and an
// offline run passes a manifest the API server refuses.
func TestBuildGlobalContextStore_RejectsBothFieldsSet(t *testing.T) {
	gce := newConfigMapGCE("my-cache", []kyvernov2.GlobalContextEntryProjection{
		{Name: "count", JMESPath: "length(@)", Expression: "object.size()"},
	})

	_, err := buildGlobalContextStore(context.Background(), []*kyvernov2.GlobalContextEntry{gce}, fakeConfigMapClient(t), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sets both jmesPath and expression")
}

func TestBuildGlobalContextStore_RejectsNeitherFieldSet(t *testing.T) {
	gce := newConfigMapGCE("my-cache", []kyvernov2.GlobalContextEntryProjection{
		{Name: "count"},
	})

	_, err := buildGlobalContextStore(context.Background(), []*kyvernov2.GlobalContextEntry{gce}, fakeConfigMapClient(t), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sets neither jmesPath nor expression")
}

// The deprecation warning is the one user-visible behaviour change in this release for
// JMESPath users, so pin both halves: that it is emitted, and that --warnings-as-errors
// turns it into a failure.
func TestRecordGlobalContextEntryDeprecationWarnings(t *testing.T) {
	jmesPathGCE := &kyvernov2.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy"},
		Spec: kyvernov2.GlobalContextEntrySpec{
			Projections: []kyvernov2.GlobalContextEntryProjection{
				{Name: "p", JMESPath: "items | length(@)"},
			},
		},
	}
	expressionGCE := &kyvernov2.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "modern"},
		Spec: kyvernov2.GlobalContextEntrySpec{
			Projections: []kyvernov2.GlobalContextEntryProjection{
				{Name: "p", Expression: "object.items.size()"},
			},
		},
	}

	t.Run("jmesPath warns and names the field path", func(t *testing.T) {
		c := &ApplyCommandConfig{}
		var out bytes.Buffer
		c.recordGlobalContextEntryDeprecationWarnings(&out, []*kyvernov2.GlobalContextEntry{jmesPathGCE})
		require.Len(t, c.deprecationWarnings, 1)
		assert.Contains(t, c.deprecationWarnings[0], "legacy")
		assert.Contains(t, c.deprecationWarnings[0], "spec.projections[0].jmesPath")
		assert.Contains(t, out.String(), "Warning: ")
	})

	t.Run("expression does not warn", func(t *testing.T) {
		c := &ApplyCommandConfig{}
		var out bytes.Buffer
		c.recordGlobalContextEntryDeprecationWarnings(&out, []*kyvernov2.GlobalContextEntry{expressionGCE})
		assert.Empty(t, c.deprecationWarnings)
		assert.Empty(t, out.String())
	})

	t.Run("warnings-as-errors turns the warning into a failure", func(t *testing.T) {
		c := &ApplyCommandConfig{warningsAsErrors: true}
		var out bytes.Buffer
		c.recordGlobalContextEntryDeprecationWarnings(&out, []*kyvernov2.GlobalContextEntry{jmesPathGCE})
		err := c.failOnDeprecationWarnings()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--warnings-as-errors is set")
	})

	t.Run("without the flag the warning is advisory", func(t *testing.T) {
		c := &ApplyCommandConfig{}
		var out bytes.Buffer
		c.recordGlobalContextEntryDeprecationWarnings(&out, []*kyvernov2.GlobalContextEntry{jmesPathGCE})
		assert.NoError(t, c.failOnDeprecationWarnings())
	})
}
