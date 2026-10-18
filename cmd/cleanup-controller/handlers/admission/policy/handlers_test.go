package policy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
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
// validation.Validate's discovery.ServerPreferredResources(client.Discovery().CachedDiscoveryInterface())
// call has a real, non-nil, non-aggregated discovery interface to walk instead of the dclient
// test fake's CachedDiscoveryInterface(), which stubs to nil and panics deeper in client-go.
type cachedFakeDiscovery struct {
	*discoveryfake.FakeDiscovery
}

func (cachedFakeDiscovery) Fresh() bool { return true }
func (cachedFakeDiscovery) Invalidate() {}

// discoveryWithCache overrides only CachedDiscoveryInterface() on top of
// dclient.NewFakeDiscoveryClient, which the rest of the fake dclient.Interface still needs for
// GetGVRFromGVK/FindResources/etc.
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

// validCleanupPolicy passes validation.Validate against an empty fake client: an empty
// MatchResources has no resource filters, so no delete/list SubjectAccessReview is required, and
// the schedule is a well-formed cron expression.
func validCleanupPolicy(name string) *kyvernov2.ClusterCleanupPolicy {
	return &kyvernov2.ClusterCleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "0 0 * * *",
		},
	}
}

func legacyCleanupPolicyKind() metav1.GroupVersionKind {
	return metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "ClusterCleanupPolicy"}
}

func newCleanupPolicyRequest(t *testing.T, operation admissionv1.Operation, obj, oldObj *kyvernov2.ClusterCleanupPolicy, subResource string) handlers.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(obj)
	require.NoError(t, err)
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:         types.UID("test-uid"),
			Kind:        legacyCleanupPolicyKind(),
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
// ClusterCleanupPolicy/CleanupPolicy (see #17483/#17484/#17486). It specifically exercises the
// status-subresource case: Kyverno's own cleanup controller writes ClusterCleanupPolicy/status
// (pkg/controllers/cleanup/controller.go), and the cleanup-controller's webhook rule matches
// "clustercleanuppolicies/*" including subresources — if the block didn't exempt subresources it
// would deadlock the cleanup controller against its own status updates.
func TestValidateBlocksLegacyWrites(t *testing.T) {
	policy := validCleanupPolicy("test-policy")
	changedPolicy := policy.DeepCopy()
	changedPolicy.Spec.Schedule = "0 12 * * *"
	metadataOnlyChange := policy.DeepCopy()
	metadataOnlyChange.ObjectMeta.ResourceVersion = "123"
	metadataOnlyChange.ObjectMeta.Labels = map[string]string{"foo": "bar"}

	tests := []struct {
		name       string
		toggleOff  bool
		request    handlers.AdmissionRequest
		allowed    bool
		wantSilent bool // if true, assert resp.Warnings is empty too, not just Allowed
	}{
		{
			name:    "create is blocked",
			request: newCleanupPolicyRequest(t, admissionv1.Create, policy, nil, ""),
			allowed: false,
		},
		{
			name:    "spec-changing update is blocked",
			request: newCleanupPolicyRequest(t, admissionv1.Update, changedPolicy, policy, ""),
			allowed: false,
		},
		{
			name:    "no-op GitOps re-apply is allowed",
			request: newCleanupPolicyRequest(t, admissionv1.Update, policy, policy, ""),
			allowed: true,
		},
		{
			name:    "metadata-only update is allowed",
			request: newCleanupPolicyRequest(t, admissionv1.Update, metadataOnlyChange, policy, ""),
			allowed: true,
		},
		{
			// This is the case that matters most for this controller: it short-circuits
			// before validation.Validate/BuildKindWarning entirely, so the cleanup
			// controller's own UpdateStatus calls never re-run full policy validation (auth
			// checks, schedule/match validation) on every reconcile, and never emit a
			// deprecation warning for its own housekeeping writes.
			name:       "status subresource update is allowed and silent (cleanup controller's own status writes)",
			request:    newCleanupPolicyRequest(t, admissionv1.Update, changedPolicy, policy, "status"),
			allowed:    true,
			wantSilent: true,
		},
		{
			name:      "toggle disabled allows create",
			toggleOff: true,
			request:   newCleanupPolicyRequest(t, admissionv1.Create, policy, nil, ""),
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

			h := New(newFakeClient())
			resp := h.Validate(context.Background(), logr.Discard(), tt.request, time.Now())
			assert.Equal(t, tt.allowed, resp.Allowed, "message: %v", resp.Result)
			if tt.wantSilent {
				assert.Empty(t, resp.Warnings)
			}
		})
	}
}
