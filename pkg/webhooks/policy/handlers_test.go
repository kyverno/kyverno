package policy

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
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

// TestValidateSkipsPoliciesOwnedByOtherControllers proves Kyverno doesn't validate, or block, a
// policy whose evaluation mode belongs to another controller, such as Kyverno Authz.
func TestValidateSkipsPoliciesOwnedByOtherControllers(t *testing.T) {
	newRequest := func(mode string) handlers.AdmissionRequest {
		// envoy.allowed() is only declared in the Kyverno Authz Envoy environment, so this policy
		// fails to compile as a Kubernetes policy
		raw := []byte(`{
			"apiVersion": "policies.kyverno.io/v1beta1",
			"kind": "ValidatingPolicy",
			"metadata": {"name": "vpol"},
			"spec": {
				"evaluation": {"mode": "` + mode + `"},
				"validations": [{"expression": "envoy.allowed()"}]
			}
		}`)
		return handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1beta1", Kind: "ValidatingPolicy"},
				Operation: admissionv1.Create,
				Object:    runtime.RawExtension{Raw: raw},
			},
		}
	}
	h := NewHandlers(newFakeClient(), nil, "", "")

	resp := h.Validate(context.Background(), logr.Discard(), newRequest("Kubernetes"), "", time.Now())
	assert.False(t, resp.Allowed, "a Kubernetes policy with an invalid expression must be rejected")

	for _, mode := range []string{"Envoy", "HTTP"} {
		resp := h.Validate(context.Background(), logr.Discard(), newRequest(mode), "", time.Now())
		assert.True(t, resp.Allowed, "mode %s: %v", mode, resp.Result)
	}
}
