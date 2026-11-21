package dclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// newWireRecordingClient wires a real dynamic client (so path.Join actually
// runs) into a dclient.Interface pointed at an httptest server, and returns
// the client plus the recorded "METHOD /path" of every request the server
// saw. This is the only double that can distinguish "rejected before any
// request" from "rejected by the fake apiserver for an unrelated reason"
// (GHSA-4pqm-wqq6-h9q2).
func newWireRecordingClient(t *testing.T, body []byte) (Interface, *[]string) {
	t.Helper()
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	dyn, err := dynamic.NewForConfig(&rest.Config{Host: srv.URL})
	require.NoError(t, err)

	disco := NewFakeDiscoveryClient([]schema.GroupVersionResource{{Version: "v1", Resource: "pods"}})
	return NewFakeClientWithDisco(dyn, nil, disco), &requests
}

func podBody(namespace, name string) []byte {
	b, _ := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"namespace": namespace, "name": name},
	})
	return b
}

func podObj(namespace, name string) *unstructured.Unstructured {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetNamespace(namespace)
	pod.SetName(name)
	return pod
}

func Test_GetResource_RejectsUnsafeSubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.GetResource(t.Context(), "v1", "Pod", "tenant-ns", "mypod", "../../../namespaces/kube-system/secrets")
	assert.Error(t, err)
	assert.Empty(t, *requests, "no request must be sent for a traversing subresource")
}

func Test_GetResource_AllowsLegitimateSubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.GetResource(t.Context(), "v1", "Pod", "tenant-ns", "mypod", "status")
	require.NoError(t, err)
	assert.Equal(t, []string{"GET /api/v1/namespaces/tenant-ns/pods/mypod/status"}, *requests)
}

func Test_GetResource_AllowsEmptySubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.GetResource(t.Context(), "v1", "Pod", "tenant-ns", "mypod", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"GET /api/v1/namespaces/tenant-ns/pods/mypod"}, *requests)
}

func Test_UpdateResource_RejectsUnsafeSubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.UpdateResource(t.Context(), "v1", "Pod", "tenant-ns", podObj("tenant-ns", "mypod"), false, "../../../namespaces/kube-system/pods/victim")
	assert.Error(t, err)
	assert.Empty(t, *requests, "no request must be sent for a traversing subresource")
}

func Test_UpdateResource_AllowsLegitimateSubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.UpdateResource(t.Context(), "v1", "Pod", "tenant-ns", podObj("tenant-ns", "mypod"), false, "status")
	require.NoError(t, err)
	assert.Equal(t, []string{"PUT /api/v1/namespaces/tenant-ns/pods/mypod/status"}, *requests)
}

func Test_ApplyResource_RejectsUnsafeSubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.ApplyResource(t.Context(), "v1", "Pod", "tenant-ns", "mypod", podObj("tenant-ns", "mypod"), false, "kyverno", "../../../namespaces/kube-system/pods/victim")
	assert.Error(t, err)
	assert.Empty(t, *requests, "no request must be sent for a traversing subresource")
}

func Test_ApplyResource_AllowsLegitimateSubresource(t *testing.T) {
	c, requests := newWireRecordingClient(t, podBody("tenant-ns", "mypod"))

	_, err := c.ApplyResource(t.Context(), "v1", "Pod", "tenant-ns", "mypod", podObj("tenant-ns", "mypod"), false, "kyverno", "status")
	require.NoError(t, err)
	assert.Equal(t, []string{"PATCH /api/v1/namespaces/tenant-ns/pods/mypod/status"}, *requests)
}
