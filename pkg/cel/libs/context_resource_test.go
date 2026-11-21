package libs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// newWireContextProvider wires a real client-go dynamic client (so
// (*rest.Request).AbsPath and path.Join actually run) into a contextProvider,
// pointed at an httptest server that records every request path it sees.
// A fake dynamic client (k8s.io/client-go/dynamic/fake) never builds a URL,
// so it cannot tell "rejected by our validator" apart from "not found in the
// tracker" - this is the only double that can (GHSA-4pqm-wqq6-h9q2).
func newWireContextProvider(t *testing.T, body []byte) (*contextProvider, *[]string) {
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

	// "pods" isn't in the fake discovery client's built-in defaults; add it
	// alongside them so both the pods/status and the configmaps cases below
	// resolve to a GVR.
	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{{Version: "v1", Resource: "pods"}})
	return &contextProvider{client: dclient.NewFakeClientWithDisco(dyn, nil, disco)}, &requests
}

func objBody(apiVersion, kind, namespace, name string) []byte {
	b, _ := json.Marshal(map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"namespace": namespace, "name": name},
	})
	return b
}

func listBody(apiVersion, kind string) []byte {
	b, _ := json.Marshal(map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"items":      []any{},
	})
	return b
}

// Test_GetResource_RejectsTraversal covers GHSA-4pqm-wqq6-h9q2: a "." or ".."
// (or any other unsafe character) hidden in the resource, subresource, name,
// namespace, or apiVersion argument must never reach the dynamic client,
// since client-go's AbsPath joins these into a single path with path.Join,
// which resolves such segments and can redirect the request to an entirely
// different resource, namespace, or API group/version. The assertion that
// matters is not just "an error came back" - it's that zero HTTP requests
// were made; without that, a case can pass for the wrong reason (e.g. a
// well-formed request that happens to 404).
//
// The name and namespace rows are defense in depth: client-go's own
// validateNamespaceWithOptionalName already rejects those, so they stay green
// even with our validation removed. Every other row here fails without it.
func Test_GetResource_RejectsTraversal(t *testing.T) {
	cases := []struct {
		name      string
		apiVer    string
		resource  string
		namespace string
		objName   string
	}{
		{"traversal via subresource escapes to another resource type", "v1", "pods/../../secrets", "tenant-ns", "top-secret"},
		{"traversal via subresource escapes the namespace", "v1", "pods/../../../namespaces/kube-system/secrets", "tenant-ns", "top-secret"},
		{"bare dot resource", "v1", ".", "tenant-ns", "mypod"},
		{"bare dotdot resource", "v1", "..", "tenant-ns", "mypod"},
		{"trailing slash on resource", "v1", "pods/", "tenant-ns", "mypod"},
		{"percent-encoded traversal in subresource", "v1", "pods/..%2f..%2fsecrets", "tenant-ns", "mypod"},
		{"traversal via object name", "v1", "pods", "tenant-ns", "../../secrets/top-secret"},
		{"traversal via namespace", "v1", "pods", "..", "mypod"},
		{"traversal via apiVersion group", "../v1", "pods", "tenant-ns", "mypod"},
		{"traversal via apiVersion version", "apps/..", "pods", "tenant-ns", "mypod"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp, requests := newWireContextProvider(t, objBody("v1", "Pod", "tenant-ns", "mypod"))
			_, err := cp.GetResource(tc.apiVer, tc.resource, tc.namespace, tc.objName)
			assert.Error(t, err)
			assert.Empty(t, *requests, "no request must be sent for a traversal input")
		})
	}
}

func Test_GetResource_AllowsLegitimateRequest(t *testing.T) {
	cp, requests := newWireContextProvider(t, objBody("v1", "ConfigMap", "tenant-ns", "local-cm"))

	res, err := cp.GetResource("v1", "configmaps", "tenant-ns", "local-cm")
	require.NoError(t, err)
	assert.Equal(t, "local-cm", res.GetName())
	assert.Equal(t, []string{"GET /api/v1/namespaces/tenant-ns/configmaps/local-cm"}, *requests)
}

func Test_GetResource_AllowsLegitimateSubresource(t *testing.T) {
	cp, requests := newWireContextProvider(t, objBody("v1", "Pod", "tenant-ns", "mypod"))

	res, err := cp.GetResource("v1", "pods/status", "tenant-ns", "mypod")
	require.NoError(t, err)
	assert.Equal(t, "mypod", res.GetName())
	assert.Equal(t, []string{"GET /api/v1/namespaces/tenant-ns/pods/mypod/status"}, *requests)
}

func Test_GetResource_AllowsClusterScopedRequest(t *testing.T) {
	cp, requests := newWireContextProvider(t, objBody("v1", "ConfigMap", "", "cluster-cm"))

	_, err := cp.GetResource("v1", "configmaps", "", "cluster-cm")
	require.NoError(t, err)
	assert.Equal(t, []string{"GET /api/v1/configmaps/cluster-cm"}, *requests)
}

// Test_ListResources_RejectsUnsafeResourceArgument covers GHSA-4pqm-wqq6-h9q2
// for List specifically. Pre-fix, List is exploitable, not merely malformed:
// the "resource" string is passed unsplit into
// groupVersion.WithResource(resource), and makeURLSegments appends it as a
// single path segment, so path.Join resolves embedded ".." the same way it
// does for Get/Post. The advisory's claim that List is unaffected is wrong;
// resource.list('v1', 'configmaps/../../../namespaces/kube-system/secrets')
// reaches /api/v1/namespaces/kube-system/secrets on the wire pre-fix.
func Test_ListResources_RejectsUnsafeResourceArgument(t *testing.T) {
	cases := []struct {
		name      string
		apiVer    string
		resource  string
		namespace string
	}{
		{"traversal via resource escapes the namespace", "v1", "configmaps/../../../namespaces/kube-system/secrets", "tenant-ns"},
		{"percent-encoded traversal in resource", "v1", "configmaps%2f..%2fsecrets", "tenant-ns"},
		{"traversal via namespace", "v1", "pods", ".."},
		{"traversal via apiVersion", "../v1", "pods", "tenant-ns"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp, requests := newWireContextProvider(t, listBody("v1", "PodList"))
			_, err := cp.ListResources(tc.apiVer, tc.resource, tc.namespace, nil)
			assert.Error(t, err)
			assert.Empty(t, *requests, "no request must be sent for a traversal input")
		})
	}
}

func Test_ListResources_AllowsLegitimateRequest(t *testing.T) {
	cp, requests := newWireContextProvider(t, listBody("v1", "PodList"))

	_, err := cp.ListResources("v1", "pods", "tenant-ns", nil)
	require.NoError(t, err)
	require.Len(t, *requests, 1)
	assert.Equal(t, "GET /api/v1/namespaces/tenant-ns/pods", (*requests)[0])
}

// Test_PostResource_RejectsTraversal covers GHSA-4pqm-wqq6-h9q2 for Post: the
// subresource position is exploitable the same way it is for Get.
func Test_PostResource_RejectsTraversal(t *testing.T) {
	cases := []struct {
		name      string
		resource  string
		namespace string
	}{
		{"traversal via subresource escapes to another resource type", "pods/../../secrets", "tenant-ns"},
		{"traversal via namespace", "pods", ".."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp, requests := newWireContextProvider(t, objBody("v1", "Pod", "tenant-ns", "created-via-traversal"))
			_, err := cp.PostResource("v1", tc.resource, tc.namespace, map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata":   map[string]any{"name": "created-via-traversal"},
			})
			assert.Error(t, err)
			assert.Empty(t, *requests, "no request must be sent for a traversal input")
		})
	}
}

func Test_PostResource_AllowsLegitimateRequest(t *testing.T) {
	cp, requests := newWireContextProvider(t, objBody("v1", "ConfigMap", "tenant-ns", "created-cm"))

	_, err := cp.PostResource("v1", "configmaps", "tenant-ns", map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "created-cm"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"POST /api/v1/namespaces/tenant-ns/configmaps"}, *requests)
}

// Test_NamespacedPolicyCannotEscapeItsNamespace covers issue #957's required
// test 3 at the layer the vulnerability is actually reachable from: the
// namespace-pinned CEL environment a NamespacedValidatingPolicy gets, where
// the author cannot pass a namespace argument at all and the only lever left
// is the resource string.
func Test_NamespacedPolicyCannotEscapeItsNamespace(t *testing.T) {
	cases := []struct {
		name string
		expr string
	}{
		{"get escapes to another namespace", `resource.get('v1','configmaps/../../../../namespaces/kube-system/configmaps/kv-canary','x')`},
		{"get escapes to cluster scope", `resource.get('v1','configmaps/../../../../../../apis/rbac.authorization.k8s.io/v1/clusterrolebindings/cluster-admin','x')`},
		{"list escapes to another namespace", `resource.list('v1','configmaps/../../../namespaces/kube-system/secrets')`},
		// metadata.name is required: client-go's dynamic Create rejects a nameless
		// object before any path building, which would make this pass vacuously
		{"post escapes to cluster scope", `resource.post('v1','configmaps/../../../../../../apis/admissionregistration.k8s.io/v1/mutatingwebhookconfigurations',{'apiVersion':dyn('admissionregistration.k8s.io/v1'),'kind':dyn('MutatingWebhookConfiguration'),'metadata':dyn({'name':dyn('tenant-owns-the-cluster')})})`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp, requests := newWireContextProvider(t, objBody("v1", "ConfigMap", "kube-system", "kv-canary"))
			out, err := evalNamespacedResourceExpr(t, cp, "tenant-ns", tc.expr)
			assert.Error(t, err, "evaluated to %v", out)
			assert.Empty(t, *requests, "no request must leave the policy's namespace")
		})
	}
}

func Test_NamespacedPolicyReadsItsOwnNamespace(t *testing.T) {
	cp, requests := newWireContextProvider(t, objBody("v1", "ConfigMap", "tenant-ns", "local-cm"))

	_, err := evalNamespacedResourceExpr(t, cp, "tenant-ns", `resource.get('v1','configmaps','local-cm')`)
	require.NoError(t, err)
	assert.Equal(t, []string{"GET /api/v1/namespaces/tenant-ns/configmaps/local-cm"}, *requests)
}

// evalNamespacedResourceExpr compiles and evaluates expr against the resource
// library bound to policyNamespace, which registers only the overloads that
// take no namespace argument.
func evalNamespacedResourceExpr(t *testing.T, cp *contextProvider, policyNamespace, expr string) (any, error) {
	t.Helper()
	env, err := cel.NewEnv(resource.Lib(resource.Context{ContextInterface: cp}, policyNamespace, resource.Latest()))
	require.NoError(t, err)
	ast, iss := env.Compile(expr)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := prg.Eval(map[string]any{})
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}
