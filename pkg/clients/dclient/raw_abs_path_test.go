package dclient

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newFakeClientWithObjects(t *testing.T, objs ...*unstructured.Unstructured) Interface {
	t.Helper()

	gvrToListKind := make(map[schema.GroupVersionResource]string)
	gvrToGVK := make(map[schema.GroupVersionResource]schema.GroupVersionKind)
	var gvrs []schema.GroupVersionResource
	seen := make(map[schema.GroupVersionResource]bool)

	runtimeObjs := make([]runtime.Object, len(objs))
	for i, o := range objs {
		runtimeObjs[i] = o.DeepCopy()
		gvk := o.GroupVersionKind()
		gvr, _ := meta.UnsafeGuessKindToResource(gvk)
		if !seen[gvr] {
			seen[gvr] = true
			gvrToListKind[gvr] = gvk.Kind + "List"
			gvrToGVK[gvr] = gvk
			gvrs = append(gvrs, gvr)
		}
	}

	c, err := NewFakeClient(runtime.NewScheme(), gvrToListKind, runtimeObjs...)
	require.NoError(t, err)

	disco := NewFakeDiscoveryClient(gvrs)
	for gvr, gvk := range gvrToGVK {
		disco.AddGVRToGVKMapping(gvr, gvk)
	}
	c.SetDiscovery(disco)
	return c
}

func newObj(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

// ── GET tests ────────────────────────────────────────────────────────────────

func TestRawAbsPath_GetNamespacedResource(t *testing.T) {
	cm := newObj("v1", "ConfigMap", "default", "app-config")
	c := newFakeClientWithObjects(t, cm)

	data, err := c.RawAbsPath(context.Background(), "/api/v1/namespaces/default/configmaps/app-config", "GET", nil)
	require.NoError(t, err)

	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &obj))
	assert.Equal(t, "app-config", obj["metadata"].(map[string]interface{})["name"])
	assert.Equal(t, "default", obj["metadata"].(map[string]interface{})["namespace"])
}

func TestRawAbsPath_GetClusterScopedResource(t *testing.T) {
	role := newObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "admin-role")
	c := newFakeClientWithObjects(t, role)

	data, err := c.RawAbsPath(context.Background(), "/apis/rbac.authorization.k8s.io/v1/clusterroles/admin-role", "GET", nil)
	require.NoError(t, err)

	var obj map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &obj))
	assert.Equal(t, "admin-role", obj["metadata"].(map[string]interface{})["name"])
}

func TestRawAbsPath_GetResourceNotFound_ReturnsError(t *testing.T) {
	c := newFakeClientWithObjects(t)

	_, err := c.RawAbsPath(context.Background(), "/api/v1/namespaces/default/configmaps/missing", "GET", nil)
	assert.Error(t, err)
}

// ── LIST tests ───────────────────────────────────────────────────────────────

func TestRawAbsPath_ListNamespacedResources(t *testing.T) {
	pdbs := []*unstructured.Unstructured{
		newObj("policy/v1", "PodDisruptionBudget", "default", "pdb-1"),
		newObj("policy/v1", "PodDisruptionBudget", "default", "pdb-2"),
		newObj("policy/v1", "PodDisruptionBudget", "other-ns", "pdb-3"),
	}
	c := newFakeClientWithObjects(t, pdbs...)

	data, err := c.RawAbsPath(context.Background(), "/apis/policy/v1/namespaces/default/poddisruptionbudgets", "GET", nil)
	require.NoError(t, err)

	var list unstructured.UnstructuredList
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Len(t, list.Items, 2, "should list only PDBs in namespace 'default'")
}

func TestRawAbsPath_ListAllNamespacesReturnsAll(t *testing.T) {
	cms := []*unstructured.Unstructured{
		newObj("v1", "ConfigMap", "ns-a", "cm-1"),
		newObj("v1", "ConfigMap", "ns-b", "cm-2"),
		newObj("v1", "ConfigMap", "ns-c", "cm-3"),
	}
	c := newFakeClientWithObjects(t, cms...)

	// path without namespace segment → list across all namespaces
	data, err := c.RawAbsPath(context.Background(), "/api/v1/configmaps", "GET", nil)
	require.NoError(t, err)

	var list unstructured.UnstructuredList
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Len(t, list.Items, 3)
}

func TestRawAbsPath_ListEmptyNamespaceReturnsEmpty(t *testing.T) {
	cm := newObj("v1", "ConfigMap", "default", "my-cm")
	c := newFakeClientWithObjects(t, cm)

	data, err := c.RawAbsPath(context.Background(), "/api/v1/namespaces/other-ns/configmaps", "GET", nil)
	require.NoError(t, err)

	var list unstructured.UnstructuredList
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Empty(t, list.Items)
}

func TestRawAbsPath_ListClusterScopedResources(t *testing.T) {
	roles := []*unstructured.Unstructured{
		newObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "role-a"),
		newObj("rbac.authorization.k8s.io/v1", "ClusterRole", "", "role-b"),
	}
	c := newFakeClientWithObjects(t, roles...)

	data, err := c.RawAbsPath(context.Background(), "/apis/rbac.authorization.k8s.io/v1/clusterroles", "GET", nil)
	require.NoError(t, err)

	var list unstructured.UnstructuredList
	require.NoError(t, json.Unmarshal(data, &list))
	assert.Len(t, list.Items, 2)
}

// ── Method restriction tests ─────────────────────────────────────────────────

func TestRawAbsPath_PostNotSupportedOnFakeClient(t *testing.T) {
	c := newFakeClientWithObjects(t)

	_, err := c.RawAbsPath(context.Background(), "/api/v1/configmaps", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

// ── Path parsing edge cases ───────────────────────────────────────────────────

func TestRawAbsPath_MissingResourceSegmentReturnsError(t *testing.T) {
	c := newFakeClientWithObjects(t)

	_, err := c.RawAbsPath(context.Background(), "/api/v1", "GET", nil)
	assert.Error(t, err)
}

func TestRawAbsPath_LeadingSlashStripped(t *testing.T) {
	cm := newObj("v1", "ConfigMap", "default", "my-cm")
	c := newFakeClientWithObjects(t, cm)

	// Path with and without leading slash must be equivalent for GET-by-name.
	withSlash, err := c.RawAbsPath(context.Background(), "/api/v1/namespaces/default/configmaps/my-cm", "GET", nil)
	require.NoError(t, err)

	withoutSlash, err := c.RawAbsPath(context.Background(), "api/v1/namespaces/default/configmaps/my-cm", "GET", nil)
	require.NoError(t, err)

	assert.Equal(t, withSlash, withoutSlash)
}
