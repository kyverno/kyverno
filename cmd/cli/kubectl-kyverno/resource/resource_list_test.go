package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUnstructuredResources_KindList(t *testing.T) {
	yamlData := []byte(`
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Service
  metadata:
    name: list-service-test
  spec:
    ports:
    - protocol: TCP
      port: 80
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: list-deployment-test
  spec:
    replicas: 1
`)

	resources, err := GetUnstructuredResources(yamlData)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "Service", resources[0].GetKind())
	assert.Equal(t, "v1", resources[0].GetAPIVersion())
	assert.Equal(t, "list-service-test", resources[0].GetName())
	assert.Equal(t, "default", resources[0].GetNamespace())

	assert.Equal(t, "Deployment", resources[1].GetKind())
	assert.Equal(t, "apps/v1", resources[1].GetAPIVersion())
	assert.Equal(t, "list-deployment-test", resources[1].GetName())
	assert.Equal(t, "default", resources[1].GetNamespace())
}

func TestGetUnstructuredResources_EmptyList(t *testing.T) {
	// Empty items slice
	yamlData := []byte(`
apiVersion: v1
kind: List
items: []
`)
	resources, err := GetUnstructuredResources(yamlData)
	require.NoError(t, err)
	assert.Empty(t, resources)

	// List without items field
	yamlDataNoItems := []byte(`
apiVersion: v1
kind: List
`)
	resources, err = GetUnstructuredResources(yamlDataNoItems)
	require.NoError(t, err)
	assert.Empty(t, resources)

	// Typed list without items field
	yamlDataTypedNoItems := []byte(`
apiVersion: v1
kind: PodList
`)
	resources, err = GetUnstructuredResources(yamlDataTypedNoItems)
	require.NoError(t, err)
	assert.Empty(t, resources)

	// Typed list with empty items slice
	yamlDataTypedEmpty := []byte(`
apiVersion: v1
kind: PodList
items: []
`)
	resources, err = GetUnstructuredResources(yamlDataTypedEmpty)
	require.NoError(t, err)
	assert.Empty(t, resources)
}

func TestGetUnstructuredResources_TypedList(t *testing.T) {
	yamlData := []byte(`
apiVersion: v1
kind: PodList
items:
- metadata:
    name: pod-1
    namespace: test-ns
- metadata:
    name: pod-2
- kind: Pod
  metadata:
    name: pod-with-kind-only
`)

	resources, err := GetUnstructuredResources(yamlData)
	require.NoError(t, err)
	require.Len(t, resources, 3)

	assert.Equal(t, "Pod", resources[0].GetKind())
	assert.Equal(t, "v1", resources[0].GetAPIVersion())
	assert.Equal(t, "pod-1", resources[0].GetName())
	assert.Equal(t, "test-ns", resources[0].GetNamespace())

	assert.Equal(t, "Pod", resources[1].GetKind())
	assert.Equal(t, "v1", resources[1].GetAPIVersion())
	assert.Equal(t, "pod-2", resources[1].GetName())
	assert.Equal(t, "default", resources[1].GetNamespace())

	assert.Equal(t, "Pod", resources[2].GetKind())
	assert.Equal(t, "v1", resources[2].GetAPIVersion(), "should inherit parent apiVersion when kind is explicitly set")
	assert.Equal(t, "pod-with-kind-only", resources[2].GetName())
	assert.Equal(t, "default", resources[2].GetNamespace())
}

func TestGetUnstructuredResources_NestedList(t *testing.T) {
	yamlData := []byte(`
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Service
  metadata:
    name: svc-1
- apiVersion: v1
  kind: List
  items:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: cm-1
`)

	resources, err := GetUnstructuredResources(yamlData)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "Service", resources[0].GetKind())
	assert.Equal(t, "svc-1", resources[0].GetName())

	assert.Equal(t, "ConfigMap", resources[1].GetKind())
	assert.Equal(t, "cm-1", resources[1].GetName())
}

func TestGetUnstructuredResources_NamespaceInheritance(t *testing.T) {
	yamlData := []byte(`
apiVersion: v1
kind: List
metadata:
  namespace: custom-ns
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm-inherited
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm-explicit
    namespace: explicit-ns
`)

	resources, err := GetUnstructuredResources(yamlData)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	assert.Equal(t, "cm-inherited", resources[0].GetName())
	assert.Equal(t, "custom-ns", resources[0].GetNamespace())

	assert.Equal(t, "cm-explicit", resources[1].GetName())
	assert.Equal(t, "explicit-ns", resources[1].GetNamespace())
}
