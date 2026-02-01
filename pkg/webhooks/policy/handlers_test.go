package policy

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// TestNewHandlers verifies that handlers are created with correct service account configuration
func TestNewHandlers(t *testing.T) {
	h := NewHandlers(nil, "bg-sa", "reports-sa")

	assert.NotNil(t, h)
	assert.Equal(t, "bg-sa", h.backgroundServiceAccountName)
	assert.Equal(t, "reports-sa", h.reportsServiceAccountName)
}

func TestNewHandlers_EmptyServiceAccounts(t *testing.T) {
	h := NewHandlers(nil, "", "")

	assert.NotNil(t, h)
	assert.Empty(t, h.backgroundServiceAccountName)
	assert.Empty(t, h.reportsServiceAccountName)
}

func TestNewHandlers_NilClient(t *testing.T) {
	h := NewHandlers(nil, "bg-sa", "reports-sa")

	assert.NotNil(t, h)
	assert.Nil(t, h.client)
}

func TestMutate_ReturnsSuccess(t *testing.T) {
	h := NewHandlers(nil, "", "")

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	response := h.Mutate(context.Background(), logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid", string(response.UID))
	assert.True(t, response.Allowed)
}

func TestMutate_DifferentUIDs(t *testing.T) {
	h := NewHandlers(nil, "", "")

	uids := []string{"uid-1", "uid-2", "uid-3"}

	for _, uid := range uids {
		t.Run(uid, func(t *testing.T) {
			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID(uid),
					Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
					Operation: admissionv1.Create,
					Object:    runtime.RawExtension{Raw: []byte(`{}`)},
				},
			}

			response := h.Mutate(context.Background(), logr.Discard(), request, "", time.Now())

			assert.Equal(t, uid, string(response.UID))
			assert.True(t, response.Allowed)
		})
	}
}

func TestMutate_DifferentOperations(t *testing.T) {
	h := NewHandlers(nil, "", "")

	t.Run("Create", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-mutate-create"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
				Operation: admissionv1.Create,
				Object:    runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Mutate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-mutate-create", string(response.UID))
		assert.True(t, response.Allowed)
	})

	t.Run("Update", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-mutate-update"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
				Operation: admissionv1.Update,
				Object:    runtime.RawExtension{Raw: []byte(`{}`)},
				OldObject: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Mutate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-mutate-update", string(response.UID))
		assert.True(t, response.Allowed)
	})

	t.Run("Delete", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-mutate-delete"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
				Operation: admissionv1.Delete,
				OldObject: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Mutate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-mutate-delete", string(response.UID))
		assert.True(t, response.Allowed)
	})
}

func TestMutate_DifferentPolicyKinds(t *testing.T) {
	kinds := []struct {
		kind    string
		version string
	}{
		{kind: "ClusterPolicy", version: "v1"},
		{kind: "Policy", version: "v1"},
		{kind: "ValidatingPolicy", version: "v2alpha1"},
		{kind: "MutatingPolicy", version: "v2alpha1"},
	}

	h := NewHandlers(nil, "", "")

	for _, k := range kinds {
		t.Run(k.kind, func(t *testing.T) {
			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("uid-" + k.kind),
					Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: k.version, Kind: k.kind},
					Operation: admissionv1.Create,
					Object:    runtime.RawExtension{Raw: []byte(`{}`)},
				},
			}

			response := h.Mutate(context.Background(), logr.Discard(), request, "", time.Now())

			assert.Equal(t, "uid-"+k.kind, string(response.UID))
			assert.True(t, response.Allowed)
		})
	}
}

func TestMutate_ContextCancellation(t *testing.T) {
	h := NewHandlers(nil, "", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-cancelled",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	// Should still return a response even with cancelled context
	response := h.Mutate(ctx, logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid-cancelled", string(response.UID))
	assert.True(t, response.Allowed)
}
