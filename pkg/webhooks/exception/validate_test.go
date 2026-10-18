package exception

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/toggle"
	validation "github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func legacyExceptionKind() metav1.GroupVersionKind {
	return metav1.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "PolicyException"}
}

func newLegacyExceptionRequest(t *testing.T, operation admissionv1.Operation, obj, oldObj *kyvernov2.PolicyException, subResource string) handlers.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(obj)
	require.NoError(t, err)
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:         types.UID("test-uid"),
			Kind:        legacyExceptionKind(),
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

// TestExceptionValidateBlocksLegacyWrites covers the 1.20 write-time hard block on legacy
// kyverno.io PolicyException (see #17483/#17484/#17486): creates and spec-changing updates are
// denied, no-op GitOps re-applies and status-only changes are allowed, and the toggle escape
// hatch restores 1.19 (warning-only) behavior.
func TestExceptionValidateBlocksLegacyWrites(t *testing.T) {
	options := validation.ValidationOptions{Enabled: true, Namespace: "*"}
	exception := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "test-exception", Namespace: "default"},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{PolicyName: "test-policy"}},
		},
	}
	changedException := exception.DeepCopy()
	changedException.Spec.Exceptions[0].PolicyName = "other-policy"
	metadataOnlyChange := exception.DeepCopy()
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
			request: newLegacyExceptionRequest(t, admissionv1.Create, exception, nil, ""),
			allowed: false,
		},
		{
			name:    "spec-changing update is blocked",
			request: newLegacyExceptionRequest(t, admissionv1.Update, changedException, exception, ""),
			allowed: false,
		},
		{
			name:    "no-op GitOps re-apply is allowed",
			request: newLegacyExceptionRequest(t, admissionv1.Update, exception, exception, ""),
			allowed: true,
		},
		{
			name:    "metadata-only update is allowed",
			request: newLegacyExceptionRequest(t, admissionv1.Update, metadataOnlyChange, exception, ""),
			allowed: true,
		},
		{
			// Legacy PolicyException has no status subresource today, so this case is
			// defensive/future-proofing, matching the same short-circuit as the other two
			// handlers: subresource writes skip validation/warnings entirely, not just the
			// legacy-policy block.
			name:       "status subresource update is allowed and silent",
			request:    newLegacyExceptionRequest(t, admissionv1.Update, changedException, exception, "status"),
			allowed:    true,
			wantSilent: true,
		},
		{
			name:      "toggle disabled allows create",
			toggleOff: true,
			request:   newLegacyExceptionRequest(t, admissionv1.Create, exception, nil, ""),
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

			h := NewHandlers(options)
			resp := h.Validate(context.Background(), logr.Discard(), tt.request, "", time.Now())
			assert.Equal(t, tt.allowed, resp.Allowed)
			if tt.wantSilent {
				assert.Empty(t, resp.Warnings)
			}
		})
	}
}
