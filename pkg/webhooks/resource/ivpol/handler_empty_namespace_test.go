package ivpol

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNamespacedHandlers_EmptyNamespace(t *testing.T) {
	// An admission request with an empty namespace (such as a cluster-scoped resource
	// routed to the namespaced endpoint) must immediately return success without
	// evaluating policies or mutating digests.
	h := &handler{}
	req := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid-empty-ns"),
			Namespace: "",
		},
	}

	t.Run("MutateNamespaced returns success immediately", func(t *testing.T) {
		resp := h.MutateNamespaced(context.Background(), logr.Discard(), req, "", time.Now())
		assert.True(t, resp.Allowed)
		assert.Equal(t, types.UID("test-uid-empty-ns"), resp.UID)
	})

	t.Run("ValidateNamespaced returns success immediately", func(t *testing.T) {
		resp := h.ValidateNamespaced(context.Background(), logr.Discard(), req, "", time.Now())
		assert.True(t, resp.Allowed)
		assert.Equal(t, types.UID("test-uid-empty-ns"), resp.UID)
	})
}
