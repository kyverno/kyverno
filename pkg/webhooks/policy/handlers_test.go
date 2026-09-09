package policy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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

// validClusterPolicyRaw is a minimal, known-valid ClusterPolicy payload (one validate rule
// matching Pods), used as the base fixture for the legacy-policy write-block tests below.
const validClusterPolicyRaw = `{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {"name": "check-labels"},
	"spec": {
		"rules": [{
			"name": "check-labels",
			"match": {"resources": {"kinds": ["Pod"]}},
			"validate": {
				"message": "label app is required",
				"pattern": {"metadata": {"labels": {"app": "?*"}}}
			}
		}]
	}
}`

func mustUnmarshalClusterPolicy(t *testing.T) *kyvernov1.ClusterPolicy {
	t.Helper()
	var policy *kyvernov1.ClusterPolicy
	require.NoError(t, json.Unmarshal([]byte(validClusterPolicyRaw), &policy))
	return policy
}

func legacyClusterPolicyKind() metav1.GroupVersionKind {
	return metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}
}

func newClusterPolicyRequest(t *testing.T, operation admissionv1.Operation, obj, oldObj *kyvernov1.ClusterPolicy, subResource string) handlers.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(obj)
	require.NoError(t, err)
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:         types.UID("test-uid"),
			Kind:        legacyClusterPolicyKind(),
			Operation:   operation,
			SubResource: subResource,
			Object:      runtime.RawExtension{Raw: raw},
		},
	}
	if oldObj != nil {
		oldRaw, err := json.Marshal(oldObj)
		require.NoError(t, err)
		request.OldObject = runtime.RawExtension{Raw: oldRaw}
	}
	return request
}

// TestValidateBlocksLegacyWrites covers the 1.20 write-time hard block on legacy
// ClusterPolicy/Policy (see #17483/#17484/#17486): creates and spec-changing updates are denied,
// no-op GitOps re-applies and metadata/status-only changes are allowed, and the
// blockLegacyPolicyAPIs toggle is the escape hatch back to 1.19 (warning-only) behavior.
func TestValidateBlocksLegacyWrites(t *testing.T) {
	policy := mustUnmarshalClusterPolicy(t)
	changedPolicy := policy.DeepCopy()
	changedPolicy.Spec.Rules[0].Validation.Message = "a different message"
	metadataOnlyChange := policy.DeepCopy()
	metadataOnlyChange.ObjectMeta.ResourceVersion = "123"
	metadataOnlyChange.ObjectMeta.Labels = map[string]string{"foo": "bar"}

	tests := []struct {
		name      string
		toggleOff bool
		request   handlers.AdmissionRequest
		allowed   bool
	}{
		{
			name:    "create is blocked",
			request: newClusterPolicyRequest(t, admissionv1.Create, policy, nil, ""),
			allowed: false,
		},
		{
			name:    "spec-changing update is blocked",
			request: newClusterPolicyRequest(t, admissionv1.Update, changedPolicy, policy, ""),
			allowed: false,
		},
		{
			name:    "no-op GitOps re-apply is allowed",
			request: newClusterPolicyRequest(t, admissionv1.Update, policy, policy, ""),
			allowed: true,
		},
		{
			name:    "metadata-only update is allowed",
			request: newClusterPolicyRequest(t, admissionv1.Update, metadataOnlyChange, policy, ""),
			allowed: true,
		},
		{
			name:    "status subresource update is allowed",
			request: newClusterPolicyRequest(t, admissionv1.Update, changedPolicy, policy, "status"),
			allowed: true,
		},
		{
			name:      "toggle disabled allows create",
			toggleOff: true,
			request:   newClusterPolicyRequest(t, admissionv1.Create, policy, nil, ""),
			allowed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse(""))
			})
			if tt.toggleOff {
				require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse("false"))
			}

			h := NewHandlers(newFakeClient(), nil, "", "")
			resp := h.Validate(context.Background(), logr.Discard(), tt.request, "", time.Now())
			assert.Equal(t, tt.allowed, resp.Allowed, "message: %v", resp.Result)
		})
	}
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
