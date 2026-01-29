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

func TestValidate_InvalidRequest(t *testing.T) {
	h := NewHandlers(nil, "", "")

	// Create an invalid admission request (not a policy)
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
	h := NewHandlers(nil, "", "")

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-empty",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	}

	response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid-empty", string(response.UID))
}

func TestValidate_NilObject(t *testing.T) {
	h := NewHandlers(nil, "", "")

	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid-nil",
			Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
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
			objectJSON:  `{"apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy"}`,
			expectedUID: "uid-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(nil, "", "")

			request := handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID(tt.expectedUID),
					Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
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
	response := h.Validate(ctx, logr.Discard(), request, "", time.Now())

	assert.Equal(t, "test-uid-cancelled", string(response.UID))
}

func TestValidate_DifferentOperations(t *testing.T) {
	h := NewHandlers(nil, "", "")

	t.Run("Create", func(t *testing.T) {
		request := handlers.AdmissionRequest{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("uid-create"),
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
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
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
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
				Kind:      metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"},
				Operation: admissionv1.Delete,
				OldObject: runtime.RawExtension{Raw: []byte(`{}`)},
			},
		}
		response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())
		assert.Equal(t, "uid-delete", string(response.UID))
	})
}

func TestValidate_DifferentPolicyKinds(t *testing.T) {
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

			response := h.Validate(context.Background(), logr.Discard(), request, "", time.Now())

			assert.Equal(t, "uid-"+k.kind, string(response.UID))
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
