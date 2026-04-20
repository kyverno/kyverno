package exception

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	validation "github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func newAdmissionRequest(t *testing.T, obj any) handlers.AdmissionRequest {
	raw, err := json.Marshal(obj)
	assert.NoError(t, err)

	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
			Object: runtime.RawExtension{
				Raw: raw,
			},
			Operation: admissionv1.Create,
		},
	}
}

func TestExceptionValidate(t *testing.T) {
	validException := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-exception",
			Namespace: "default",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "test-policy",
				},
			},
		},
	}

	tests := []struct {
		name    string
		options validation.ValidationOptions
		request handlers.AdmissionRequest
		allowed bool
		hasWarn bool
	}{
		{
			name: "valid exception with matching namespace",
			options: validation.ValidationOptions{
				Enabled:   true,
				Namespace: "default",
			},
			request: newAdmissionRequest(t, validException),
			allowed: true,
			hasWarn: false,
		},
		{
			name: "exception disabled produces warning",
			options: validation.ValidationOptions{
				Enabled: false,
			},
			request: newAdmissionRequest(t, validException),
			allowed: true,
			hasWarn: true,
		},
		{
			name: "namespace mismatch produces warning",
			options: validation.ValidationOptions{
				Enabled:   true,
				Namespace: "other",
			},
			request: newAdmissionRequest(t, validException),
			allowed: true,
			hasWarn: true,
		},
		{
			name: "empty exception spec is allowed",
			options: validation.ValidationOptions{
				Enabled:   true,
				Namespace: "default",
			},
			request: newAdmissionRequest(t, &kyvernov2.PolicyException{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-exception",
					Namespace: "default",
				},
				Spec: kyvernov2.PolicyExceptionSpec{},
			}),
			allowed: true,
			hasWarn: false,
		},
		{
			name: "exception without policyName is rejected",
			options: validation.ValidationOptions{
				Enabled: true,
			},
			request: newAdmissionRequest(t, &kyvernov2.PolicyException{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-exception",
					Namespace: "default",
				},
				Spec: kyvernov2.PolicyExceptionSpec{
					Exceptions: []kyvernov2.Exception{
						{}, // missing PolicyName → INVALID
					},
				},
			}),
			allowed: false,
			hasWarn: true,
		},
		{
			name: "unmarshal error denies request",
			options: validation.ValidationOptions{
				Enabled: true,
			},
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID: types.UID("bad-uid"),
					Object: runtime.RawExtension{
						Raw: []byte("{invalid-json"),
					},
				},
			},
			allowed: false,
			hasWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(tt.options)

			resp := h.Validate(
				context.Background(),
				logr.Discard(),
				tt.request,
				"",
				time.Now(),
			)

			assert.Equal(t, tt.allowed, resp.Allowed)

			if tt.hasWarn {
				assert.NotEmpty(t, resp.Warnings)
			} else {
				assert.Empty(t, resp.Warnings)
			}
		})
	}
}
