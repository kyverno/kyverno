package webhook

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestLegacyStatusWriterAllowsStatusSubresource is Gap 2 from the #17708 test plan: nothing
// previously observed that this package's legacy_status.go init() actually registers "status" as
// a transitionally allowed subresource. init() has already run by the time this test executes
// (it runs on package load, not on test invocation), so this asserts the post-registration state
// rather than toggling it -- deprecations' registries are add-only by design, with no unregister
// API to drive a before/after comparison here.
func TestLegacyStatusWriterAllowsStatusSubresource(t *testing.T) {
	kind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}

	decision, err := deprecations.DecideLegacyWrite(admissionv1.AdmissionRequest{
		Kind: kind, Operation: admissionv1.Update, SubResource: "status",
	})
	assert.NotEqual(t, deprecations.Deny, decision, "legacy_status.go's init() must have registered status as allowed")
	assert.NoError(t, err)

	// Not a blanket subresource bypass: an unregistered subresource stays denied.
	decision, err = deprecations.DecideLegacyWrite(admissionv1.AdmissionRequest{
		Kind: kind, Operation: admissionv1.Update, SubResource: "scale",
	})
	assert.Equal(t, deprecations.Deny, decision, "only status is registered; scale must still be denied")
	require.Error(t, err)
}
