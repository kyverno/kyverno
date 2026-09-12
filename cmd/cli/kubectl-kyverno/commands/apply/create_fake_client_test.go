package apply

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

func makeUnstructuredWithData(t *testing.T, apiVersion, kind, namespace, name string, data map[string]interface{}) *unstructured.Unstructured {
	t.Helper()
	u := makeUnstructured(apiVersion, kind, namespace, name)
	err := unstructured.SetNestedField(u.Object, data, "data")
	require.NoError(t, err)
	return u
}

func TestCreateFakeClientFromResources_EmptyInput(t *testing.T) {
	client, err := createFakeClientFromResources(nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestCreateFakeClientFromResources_SingleResource(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("v1", "ConfigMap", "default", "my-cm"),
	}

	client, err := createFakeClientFromResources(resources, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)

	obj, err := client.GetResource(context.Background(), "v1", "ConfigMap", "default", "my-cm")
	require.NoError(t, err)
	assert.Equal(t, "my-cm", obj.GetName())
	assert.Equal(t, "default", obj.GetNamespace())
}

func TestCreateFakeClientFromResources_MultipleKinds(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("apps/v1", "Deployment", "default", "my-deploy"),
		makeUnstructured("v1", "ConfigMap", "default", "my-cm"),
		makeUnstructured("policy/v1", "PodDisruptionBudget", "default", "my-pdb"),
	}

	client, err := createFakeClientFromResources(resources, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)

	deploy, err := client.GetResource(context.Background(), "apps/v1", "Deployment", "default", "my-deploy")
	require.NoError(t, err)
	assert.Equal(t, "my-deploy", deploy.GetName())

	cm, err := client.GetResource(context.Background(), "v1", "ConfigMap", "default", "my-cm")
	require.NoError(t, err)
	assert.Equal(t, "my-cm", cm.GetName())

	pdb, err := client.GetResource(context.Background(), "policy/v1", "PodDisruptionBudget", "default", "my-pdb")
	require.NoError(t, err)
	assert.Equal(t, "my-pdb", pdb.GetName())
}

func TestCreateFakeClientFromResources_ListResourcesInNamespace(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("policy/v1", "PodDisruptionBudget", "default", "pdb-a"),
		makeUnstructured("policy/v1", "PodDisruptionBudget", "default", "pdb-b"),
		makeUnstructured("policy/v1", "PodDisruptionBudget", "other-ns", "pdb-c"),
	}

	client, err := createFakeClientFromResources(resources, nil, nil)
	require.NoError(t, err)

	list, err := client.ListResource(context.Background(), "policy/v1", "PodDisruptionBudget", "default", nil)
	require.NoError(t, err)
	assert.Len(t, list.Items, 2, "should list only PDBs in the default namespace")
}

func TestCreateFakeClientFromResources_IncludesTargetAndParamResources(t *testing.T) {
	mainResources := []*unstructured.Unstructured{
		makeUnstructured("apps/v1", "Deployment", "default", "my-deploy"),
	}
	targetResources := []*unstructured.Unstructured{
		makeUnstructured("v1", "ConfigMap", "default", "target-cm"),
	}
	paramResources := []*unstructured.Unstructured{
		makeUnstructured("v1", "ConfigMap", "default", "param-cm"),
	}

	client, err := createFakeClientFromResources(mainResources, targetResources, paramResources)
	require.NoError(t, err)

	_, err = client.GetResource(context.Background(), "apps/v1", "Deployment", "default", "my-deploy")
	require.NoError(t, err)

	_, err = client.GetResource(context.Background(), "v1", "ConfigMap", "default", "target-cm")
	require.NoError(t, err)

	_, err = client.GetResource(context.Background(), "v1", "ConfigMap", "default", "param-cm")
	require.NoError(t, err)
}

func TestCreateFakeClientFromResources_ClusterScopedResource(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeUnstructured("rbac.authorization.k8s.io/v1", "ClusterRole", "", "my-role"),
	}

	client, err := createFakeClientFromResources(resources, nil, nil)
	require.NoError(t, err)

	role, err := client.GetResource(context.Background(), "rbac.authorization.k8s.io/v1", "ClusterRole", "", "my-role")
	require.NoError(t, err)
	assert.Equal(t, "my-role", role.GetName())
}

func TestCreateFakeClientFromResources_DeduplicatesGVRRegistration(t *testing.T) {
	// Multiple resources of the same kind must not cause duplicate GVR registration
	// (which would panic in the fake dynamic client).
	resources := []*unstructured.Unstructured{
		makeUnstructured("apps/v1", "Deployment", "default", "deploy-1"),
		makeUnstructured("apps/v1", "Deployment", "default", "deploy-2"),
		makeUnstructured("apps/v1", "Deployment", "other-ns", "deploy-3"),
	}

	client, err := createFakeClientFromResources(resources, nil, nil)
	require.NoError(t, err)

	list, err := client.ListResource(context.Background(), "apps/v1", "Deployment", "default", nil)
	require.NoError(t, err)
	assert.Len(t, list.Items, 2)
}

func TestCreateFakeClientFromResources_ResourceDataPreserved(t *testing.T) {
	data := map[string]interface{}{
		"environment": "production",
		"replicas":    "3",
	}
	resources := []*unstructured.Unstructured{
		makeUnstructuredWithData(t, "v1", "ConfigMap", "default", "app-config", data),
	}

	client, err := createFakeClientFromResources(resources, nil, nil)
	require.NoError(t, err)

	obj, err := client.GetResource(context.Background(), "v1", "ConfigMap", "default", "app-config")
	require.NoError(t, err)

	gotData, found, err := unstructured.NestedMap(obj.Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "production", gotData["environment"])
	assert.Equal(t, "3", gotData["replicas"])
}

func TestCreateFakeClientFromResources_ListObject(t *testing.T) {
	listObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "List",
			"items": []interface{}{
				map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "cm-in-list",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"key": "value",
					},
				},
				map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "deploy-in-list",
						"namespace": "default",
					},
				},
			},
		},
	}

	client, err := createFakeClientFromResources([]*unstructured.Unstructured{listObj}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)

	cm, err := client.GetResource(context.Background(), "v1", "ConfigMap", "default", "cm-in-list")
	require.NoError(t, err)
	assert.Equal(t, "cm-in-list", cm.GetName())

	deploy, err := client.GetResource(context.Background(), "apps/v1", "Deployment", "default", "deploy-in-list")
	require.NoError(t, err)
	assert.Equal(t, "deploy-in-list", deploy.GetName())
}

func TestCreateFakeClientFromResources_EmptyList(t *testing.T) {
	listObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "List",
			"items":      []interface{}{},
		},
	}

	client, err := createFakeClientFromResources([]*unstructured.Unstructured{listObj}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Typed list with empty items slice
	typedListObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PodList",
			"items":      []interface{}{},
		},
	}
	client, err = createFakeClientFromResources([]*unstructured.Unstructured{typedListObj}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Typed list without items field
	typedListNoItems := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PodList",
		},
	}
	client, err = createFakeClientFromResources([]*unstructured.Unstructured{typedListNoItems}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestFlattenResources_NilAndNested(t *testing.T) {
	res, err := flattenResources(nil)
	require.NoError(t, err)
	assert.Nil(t, res)

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "cm-nested",
			},
		},
	}
	nestedList := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "List",
			"items": []interface{}{
				map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "List",
					"items": []interface{}{
						cm.Object,
					},
				},
			},
		},
	}

	flat, err := flattenResources([]*unstructured.Unstructured{nil, nestedList})
	require.NoError(t, err)
	require.Len(t, flat, 1)
	assert.Equal(t, "cm-nested", flat[0].GetName())
}

func TestFlattenResources_NonListWithItems(t *testing.T) {
	widget := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata": map[string]interface{}{
				"name": "widget-1",
			},
			"items": []interface{}{
				map[string]interface{}{"foo": "bar"},
			},
		},
	}
	flat, err := flattenResources([]*unstructured.Unstructured{widget})
	require.NoError(t, err)
	require.Len(t, flat, 1)
	assert.Equal(t, "Widget", flat[0].GetKind())
	assert.Equal(t, "widget-1", flat[0].GetName())
}
