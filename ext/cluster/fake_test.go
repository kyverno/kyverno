package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDClient_ListDoesNotPanicWithWildcardMutateExisting(t *testing.T) {
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName("default")
	ns.SetLabels(map[string]string{"context": "invalid-value"})

	cluster := fakeCluster{}

	// DClient must not panic when building the client.
	var client interface{}
	require.NotPanics(t, func() {
		var err error
		client, err = cluster.DClient([]runtime.Object{ns})
		require.NoError(t, err)
	}, "DClient() must not panic when registering Namespace objects")

	assert.NotNil(t, client)
}

// TestDClient_ListNamespacesSucceeds verifies that after DClient() is built
// with Namespace objects, a .List() call on namespaces completes without
// panicking and returns the expected resources.
func TestDClient_ListNamespacesSucceeds(t *testing.T) {
	ns1 := &unstructured.Unstructured{}
	ns1.SetAPIVersion("v1")
	ns1.SetKind("Namespace")
	ns1.SetName("default")

	ns2 := &unstructured.Unstructured{}
	ns2.SetAPIVersion("v1")
	ns2.SetKind("Namespace")
	ns2.SetName("kube-system")

	cluster := fakeCluster{}
	client, err := cluster.DClient([]runtime.Object{ns1, ns2})
	require.NoError(t, err)

	// This .List() call is exactly what panicked before the fix.
	// The engine calls this when evaluating mutateExisting with name: "*".
	var list *unstructured.UnstructuredList
	require.NotPanics(t, func() {
		list, err = client.ListResource(context.Background(), "v1", "Namespace", "", nil)
	}, "ListResource on Namespace must not panic")

	require.NoError(t, err)
	assert.Len(t, list.Items, 2)
}

// TestDClient_MultipleResourceTypesDoNotPanic verifies that DClient handles
// multiple different resource types without panicking, including when the same
// kind appears more than once (deduplication path).
func TestDClient_MultipleResourceTypesDoNotPanic(t *testing.T) {
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName("default")

	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("my-pod")
	pod.SetNamespace("default")

	ns2 := &unstructured.Unstructured{}
	ns2.SetAPIVersion("v1")
	ns2.SetKind("Namespace")
	ns2.SetName("kube-system")

	cluster := fakeCluster{}

	require.NotPanics(t, func() {
		client, err := cluster.DClient([]runtime.Object{ns, pod, ns2})
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

// TestDClient_EmptyObjectListDoesNotPanic ensures a trivial empty case works.
func TestDClient_EmptyObjectListDoesNotPanic(t *testing.T) {
	cluster := fakeCluster{}
	require.NotPanics(t, func() {
		client, err := cluster.DClient([]runtime.Object{})
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

// TestDClient_ConfigMapListDoesNotPanic verifies the fix works for resource
// types other than Namespace (general non-CRD resource path).
func TestDClient_ConfigMapListDoesNotPanic(t *testing.T) {
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetName("my-config")
	cm.SetNamespace("default")

	cluster := fakeCluster{}
	client, err := cluster.DClient([]runtime.Object{cm})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, err = client.ListResource(context.Background(), "v1", "ConfigMap", "default", nil)
	}, "ListResource on ConfigMap must not panic")

	assert.NoError(t, err)
}
