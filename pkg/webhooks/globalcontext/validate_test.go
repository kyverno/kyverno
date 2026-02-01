package globalcontext

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

// TestNewHandlers verifies that handlers are created successfully
func TestNewHandlers(t *testing.T) {
	h := NewHandlers()

	assert.NotNil(t, h)
}

func TestValidate_InvalidRequest(t *testing.T) {
	h := NewHandlers()

	// Create an invalid admission request (not a GlobalContextEntry)
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Name:      "test-pod",
			Namespace: "default",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{"invalid": "json"}`)},
		},
	}

	response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())

	// Should return a response with the UID
	assert.Equal(t, "test-uid", string(response.UID))
}

func TestValidate_EmptyObject(t *testing.T) {
	h := NewHandlers()

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-empty",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid-empty", string(response.UID))
}

func TestValidate_NilObject(t *testing.T) {
	h := NewHandlers()

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-nil",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{},
		},
	}

	response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid-nil", string(response.UID))
}

func TestValidate_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name        string
		objectJSON  string
		expectedUID string
	}{
		{
			name:        "empty object",
			objectJSON:  `{}`,
			expectedUID: "uid-1",
		},
		{
			name:        "object with spec",
			objectJSON:  `{"spec": {}}`,
			expectedUID: "uid-2",
		},
		{
			name:        "object with apiVersion and kind",
			objectJSON:  `{"apiVersion": "kyverno.io/v2alpha1", "kind": "GlobalContextEntry"}`,
			expectedUID: "uid-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers()

			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID(tt.expectedUID),
					Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
					Operation: admissionv1.Create,
					Object:    runtime.RawExtension{Raw: []byte(tt.objectJSON)},
				},
			}

			response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())

			assert.Equal(t, tt.expectedUID, string(response.UID))
		})
	}
}

func TestValidate_ContextCancellation(t *testing.T) {
	h := NewHandlers()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-cancelled",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	// Should still return a response even with cancelled context
	response := h.Validate(ctx, logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid-cancelled", string(response.UID))
}

func TestValidate_DifferentOperations(t *testing.T) {
	h := NewHandlers()

	t.Run("Create", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-create"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
				Operation: admissionv1.Create,
				Object:    runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-create", string(response.UID))
	})

	t.Run("Update", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-update"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
				Operation: admissionv1.Update,
				Object:    runtime.RawExtension{Raw: []byte(`{}`)},
				OldObject: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-update", string(response.UID))
	})

	t.Run("Delete", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-delete"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
				Operation: admissionv1.Delete,
				OldObject: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-delete", string(response.UID))
	})
}

func TestValidate_WithDifferentRequestTimes(t *testing.T) {
	h := NewHandlers()

	times := []time.Time{
		time.Now(),
		time.Now().Add(-1 * time.Hour),
		time.Now().Add(1 * time.Hour),
	}

	for i, tm := range times {
		t.Run("time_variant_"+string(rune('a'+i)), func(t *testing.T) {
			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("uid-time-" + string(rune('a'+i))),
					Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2alpha1", Kind: "GlobalContextEntry"},
					Operation: admissionv1.Create,
					Object:    runtime.RawExtension{Raw: []byte(`{}`)},
				},
			}

			response := h.Validate(context.Background(), logr.Discard(), request, "", tm)

			assert.Equal(t, "uid-time-"+string(rune('a'+i)), string(response.UID))
		})
	}
}
