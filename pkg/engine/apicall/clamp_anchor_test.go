package apicall

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"gotest.tools/v3/assert"
)

// The namespace clamp used to match /namespaces/<ns>/ anywhere in the path. A
// cluster-scoped path that merely carried the policy's own namespace further
// along therefore satisfied it, because the API server routes on the namespace's
// structural position and not on the first occurrence of the word.
func Test_NamespaceClampIsPositionallyAnchored(t *testing.T) {
	tests := []struct {
		name    string
		urlPath string
	}{{
		name:    "node proxy subresource carrying a namespace in its forwarded tail",
		urlPath: "/api/v1/nodes/worker-1/proxy/namespaces/default/configz",
	}, {
		name:    "cluster-wide core collection with a namespace segment appended",
		urlPath: "/api/v1/secrets/namespaces/default",
	}, {
		name:    "cluster-wide group collection with a namespace segment appended",
		urlPath: "/apis/apps/v1/deployments/namespaces/default",
	}, {
		name:    "synthetic namespace tail after an arbitrary core segment",
		urlPath: "/api/v1/nodes/namespaces/default/anything",
	}, {
		name:    "namespace collection rather than a single namespace",
		urlPath: "/api/v1/namespaces",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := kyvernov1.ContextEntry{
				Name: "test",
				APICall: &kyvernov1.ContextAPICall{
					APICall: kyvernov1.APICall{URLPath: tt.urlPath, Method: "GET"},
				},
			}
			call, err := New(logr.Discard(), jp, entry, enginecontext.NewContext(jp), &mockClient{}, apiConfig, "default")
			assert.NilError(t, err)
			_, err = call.Fetch(context.TODO())
			assert.ErrorContains(t, err, "does not contain a namespace segment, which is required for namespaced policies")

			// The same path stays available to a cluster-scoped policy, which is
			// not clamped at all.
			call, err = New(logr.Discard(), jp, entry, enginecontext.NewContext(jp), &mockClient{}, apiConfig, "")
			assert.NilError(t, err)
			_, err = call.Fetch(context.TODO())
			assert.NilError(t, err)
		})
	}
}

// Anchoring must not narrow what a namespaced policy can legitimately read in
// its own namespace.
func Test_NamespaceClampAllowsOwnNamespacePaths(t *testing.T) {
	allowed := []string{
		"/api/v1/namespaces/default/secrets",
		"/api/v1/namespaces/default/secrets/my-secret",
		"/api/v1/namespaces/default/pods/web/log",
		"/apis/apps/v1/namespaces/default/deployments",
		"/apis/apps/v1/namespaces/default/deployments/web",
		"/apis/apps/v1/namespaces/default/deployments/web/scale",
	}

	for _, urlPath := range allowed {
		t.Run(urlPath, func(t *testing.T) {
			entry := kyvernov1.ContextEntry{
				Name: "test",
				APICall: &kyvernov1.ContextAPICall{
					APICall: kyvernov1.APICall{URLPath: urlPath, Method: "GET"},
				},
			}
			call, err := New(logr.Discard(), jp, entry, enginecontext.NewContext(jp), &mockClient{}, apiConfig, "default")
			assert.NilError(t, err)
			_, err = call.Fetch(context.TODO())
			assert.NilError(t, err)
		})
	}
}

// Anchoring must not weaken the cross-namespace rejection the clamp exists for.
func Test_NamespaceClampStillRejectsOtherNamespaces(t *testing.T) {
	for _, urlPath := range []string{
		"/api/v1/namespaces/kube-system/secrets",
		"/apis/apps/v1/namespaces/kube-system/deployments",
	} {
		t.Run(urlPath, func(t *testing.T) {
			entry := kyvernov1.ContextEntry{
				Name: "test",
				APICall: &kyvernov1.ContextAPICall{
					APICall: kyvernov1.APICall{URLPath: urlPath, Method: "GET"},
				},
			}
			call, err := New(logr.Discard(), jp, entry, enginecontext.NewContext(jp), &mockClient{}, apiConfig, "default")
			assert.NilError(t, err)
			_, err = call.Fetch(context.TODO())
			assert.ErrorContains(t, err, "refers to namespace kube-system, which is different from the policy namespace default")
		})
	}
}
