package policy

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

// cachedFakeDiscovery adapts a plain discoveryfake.FakeDiscovery into a
// discovery.CachedDiscoveryInterface (adding no-op Fresh/Invalidate), so
// policyvalidate.Validate's discovery.ServerPreferredResources(client.Discovery().CachedDiscoveryInterface())
// call has a real, non-nil, non-aggregated discovery interface to walk instead of the dclient
// test fake's CachedDiscoveryInterface(), which stubs to nil and panics deeper in client-go.
type cachedFakeDiscovery struct {
	*discoveryfake.FakeDiscovery
}

func (cachedFakeDiscovery) Fresh() bool { return true }
func (cachedFakeDiscovery) Invalidate() {}

// discoveryWithCache overrides only CachedDiscoveryInterface() on top of
// dclient.NewFakeDiscoveryClient, which the rest of the fake dclient.Interface still needs.
type discoveryWithCache struct {
	dclient.IDiscovery
	cached discovery.CachedDiscoveryInterface
}

func (d discoveryWithCache) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return d.cached
}

func newFakeClient() dclient.Interface {
	fakeDisco := &discoveryfake.FakeDiscovery{Fake: &kubetesting.Fake{}}
	disco := discoveryWithCache{
		IDiscovery: dclient.NewFakeDiscoveryClient(nil),
		cached:     cachedFakeDiscovery{fakeDisco},
	}
	return dclient.NewFakeClientWithDisco(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), kubefake.NewSimpleClientset(), disco)
}

// TestValidateCELPoliciesUnaffected proves the legacy-policy write block never engages for the
// CEL-based policies.kyverno.io kinds handled earlier in Validate's dispatch chain.
func TestValidateCELPoliciesUnaffected(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse(""))
	})

	raw := []byte(`{
		"apiVersion": "policies.kyverno.io/v1beta1",
		"kind": "ValidatingPolicy",
		"metadata": {"name": "vpol"},
		"spec": {
			"validationActions": ["Deny"],
			"matchConstraints": {"resourceRules": [{"apiGroups": [""], "apiVersions": ["v1"], "resources": ["pods"], "operations": ["CREATE"]}]},
			"validations": [{"expression": "true"}]
		}
	}`)
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Kind:      metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1beta1", Kind: "ValidatingPolicy"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}

	h := NewHandlers(newFakeClient(), nil, "", "")
	resp := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())
	assert.True(t, resp.Allowed, "message: %v", resp.Result)
}
