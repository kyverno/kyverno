package libs

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newResourceTestProvider(t *testing.T, objects ...*unstructured.Unstructured) *contextProvider {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PodList"}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "SecretList"}, &unstructured.UnstructuredList{})
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:    "PodList",
		{Group: "", Version: "v1", Resource: "secrets"}: "SecretList",
	}
	objs := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		objs = append(objs, o)
	}
	fakeClient, err := dclient.NewFakeClient(scheme, gvrToListKind, objs...)
	require.NoError(t, err)
	fakeClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	return &contextProvider{client: fakeClient}
}

func podObject(namespace, name string) *unstructured.Unstructured {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetNamespace(namespace)
	pod.SetName(name)
	return pod
}

// Test_GetResource_RejectsTraversal covers GHSA-4pqm-wqq6-h9q2: a "." or ".."
// hidden in the resource, subresource, or name argument must never reach the
// dynamic client, since client-go's AbsPath joins these into a single path
// with path.Join, which resolves such segments and can redirect the request
// to an entirely different resource, namespace, or API group/version.
func Test_GetResource_RejectsTraversal(t *testing.T) {
	cp := newResourceTestProvider(t, podObject("default", "mypod"))

	cases := []struct {
		name      string
		apiVer    string
		resource  string
		namespace string
		objName   string
	}{
		{"traversal via subresource escapes to another resource type", "v1", "pods/../../secrets", "default", "top-secret"},
		{"traversal via subresource escapes the namespace", "v1", "pods/../../../namespaces/kube-system/secrets", "default", "top-secret"},
		{"dot resource name", "v1", ".", "default", "mypod"},
		{"dotdot resource name", "v1", "..", "default", "mypod"},
		{"traversal via object name", "v1", "pods", "default", "../../secrets/top-secret"},
		{"traversal via namespace", "v1", "pods", "..", "mypod"},
		{"traversal via apiVersion group", "../v1", "pods", "default", "mypod"},
		{"traversal via apiVersion version", "apps/..", "pods", "default", "mypod"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cp.GetResource(tc.apiVer, tc.resource, tc.namespace, tc.objName)
			assert.Error(t, err)
		})
	}
}

func Test_GetResource_AllowsLegitimateRequest(t *testing.T) {
	cp := newResourceTestProvider(t, podObject("default", "mypod"))

	res, err := cp.GetResource("v1", "pods", "default", "mypod")
	require.NoError(t, err)
	assert.Equal(t, "mypod", res.GetName())
}

func Test_ListResources_RejectsUnsafeResourceArgument(t *testing.T) {
	cp := newResourceTestProvider(t, podObject("default", "mypod"))

	// List never supports a subresource path - any embedded "/" must be
	// rejected outright rather than silently forwarded to the client.
	_, err := cp.ListResources("v1", "pods/../secrets", "default", nil)
	assert.Error(t, err)

	_, err = cp.ListResources("v1", "pods", "..", nil)
	assert.Error(t, err)

	_, err = cp.ListResources("../v1", "pods", "default", nil)
	assert.Error(t, err)
}

func Test_ListResources_AllowsLegitimateRequest(t *testing.T) {
	cp := newResourceTestProvider(t, podObject("default", "mypod"))

	list, err := cp.ListResources("v1", "pods", "default", nil)
	require.NoError(t, err)
	assert.Len(t, list.Items, 1)
}

func Test_PostResource_RejectsTraversal(t *testing.T) {
	cp := newResourceTestProvider(t)

	_, err := cp.PostResource("v1", "pods/../../secrets", "default", map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "created-via-traversal"},
	})
	assert.Error(t, err)

	_, err = cp.PostResource("v1", "pods", "..", map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "created-via-traversal"},
	})
	assert.Error(t, err)
}
