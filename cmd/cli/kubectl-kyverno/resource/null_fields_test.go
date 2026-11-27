package resource

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestYamlToUnstructuredPreservesCustomNullFields(t *testing.T) {
	t.Parallel()
	resource, err := YamlToUnstructured([]byte(`apiVersion: example.com/v1
kind: Document
metadata:
  name: example
  labels: null
data: null
spec:
  data: null
  annotations: null
  labels: null
  matchLabels: null
  nodeSelector: null
  stringData: null
  entries:
  - data: null
`))
	require.NoError(t, err)
	require.NotNil(t, resource.GetLabels(), "ObjectMeta normalization must still apply")
	require.Nil(t, resource.Object["data"], "top-level custom data must remain null")
	for _, name := range []string{"data", "annotations", "labels", "matchLabels", "nodeSelector", "stringData"} {
		value, found, err := unstructured.NestedFieldNoCopy(resource.Object, "spec", name)
		require.NoError(t, err)
		require.True(t, found, "spec.%s must remain present", name)
		require.Nil(t, value, "spec.%s must retain its explicit null value", name)
	}
	entries, found, err := unstructured.NestedSlice(resource.Object, "spec", "entries")
	require.NoError(t, err)
	require.True(t, found)
	require.Nil(t, entries[0].(map[string]interface{})["data"])
}

func TestGetUnstructuredResourcesNormalizesBuiltInListItems(t *testing.T) {
	t.Parallel()
	resources, err := GetUnstructuredResources([]byte(`apiVersion: v1
kind: ConfigMapList
items:
- metadata:
    name: example
    labels: null
  data: null
`))
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Equal(t, "v1", resources[0].GetAPIVersion())
	require.Equal(t, "ConfigMap", resources[0].GetKind())
	require.NotNil(t, resources[0].GetLabels())
	data, found, err := unstructured.NestedMap(resources[0].Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, data)
	require.Empty(t, data)
}

func TestGetUnstructuredResourcesPreservesCustomNullFieldsInLists(t *testing.T) {
	t.Parallel()
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%t", nested), func(t *testing.T) {
			t.Parallel()
			custom := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "Document",
				"metadata": map[string]interface{}{
					"name":        "example",
					"annotations": nil,
				},
				"spec": map[string]interface{}{"data": nil},
			}}
			item := custom.Object
			if nested {
				item = map[string]interface{}{
					"apiVersion": "v1", "kind": "List", "items": []interface{}{item},
				}
			}
			list := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "List", "items": []interface{}{item},
			}}
			content, err := list.MarshalJSON()
			require.NoError(t, err)
			resources, err := GetUnstructuredResources(content)
			require.NoError(t, err)
			require.Len(t, resources, 1)
			value, found, err := unstructured.NestedFieldNoCopy(resources[0].Object, "spec", "data")
			require.NoError(t, err)
			require.True(t, found)
			require.Nil(t, value)
			require.NotNil(t, resources[0].GetAnnotations(), "metadata normalization must still apply")
		})
	}
}
