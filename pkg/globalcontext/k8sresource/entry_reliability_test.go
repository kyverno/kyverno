package k8sresource

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// Regression for #16949: neither a lister object nor a returned snapshot may
// alias the other, including nested JSON maps and arrays.
func TestEntry_ListObjects_IsolatesNestedCacheObjects(t *testing.T) {
	source := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "before"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{"name": "original"},
			},
		},
	}}
	e := &entry{lister: &mockLister{objects: []runtime.Object{source}}}

	objects, err := e.Get("")
	require.NoError(t, err)
	items, ok := objects.([]interface{})
	require.True(t, ok)
	require.Len(t, items, 1)
	snapshot, ok := items[0].(map[string]interface{})
	require.True(t, ok)

	source.Object["metadata"].(map[string]interface{})["name"] = "changed in cache"
	source.Object["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["name"] = "changed container"
	require.Equal(t, "before", snapshot["metadata"].(map[string]interface{})["name"])
	require.Equal(t, "original", snapshot["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["name"])

	snapshot["metadata"].(map[string]interface{})["name"] = "changed by reader"
	snapshot["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["name"] = "changed by reader"
	require.Equal(t, "changed in cache", source.Object["metadata"].(map[string]interface{})["name"])
	require.Equal(t, "changed container", source.Object["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["name"])

	next, err := e.Get("")
	require.NoError(t, err)
	require.Equal(t, "changed in cache", next.([]interface{})[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"])
}
