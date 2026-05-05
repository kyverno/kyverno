package globalcontext

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

func admissionRequestWithObject(t *testing.T, obj any) handlers.AdmissionRequest {
	t.Helper()

	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal object: %v", err)
	}

	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID: "test-uid",
			Object: runtime.RawExtension{
				Raw: raw,
			},
		},
	}
}

func TestGlobalContextValidate(t *testing.T) {
	handler := NewHandlers()
	ctx := context.Background()
	logger := logr.Discard()
	now := time.Now()

	validGctx := &kyvernov2beta1.GlobalContextEntry{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalContextEntry",
			APIVersion: "kyverno.io/v2beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-entry",
		},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			KubernetesResource: &kyvernov2beta1.KubernetesResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
		},
	}

	invalidGctx := &kyvernov2beta1.GlobalContextEntry{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalContextEntry",
			APIVersion: "kyverno.io/v2beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-entry",
		},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{},
	}

	tests := []struct {
		name      string
		request   handlers.AdmissionRequest
		wantAllow bool
	}{
		{
			name:      "valid global context entry is allowed",
			request:   admissionRequestWithObject(t, validGctx),
			wantAllow: true,
		},
		{
			name:      "invalid global context entry is rejected",
			request:   admissionRequestWithObject(t, invalidGctx),
			wantAllow: false,
		},
		{
			name: "unmarshal error rejects request",
			request: handlers.AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID: "bad-uid",
					Object: runtime.RawExtension{
						Raw: []byte(`{invalid-json`),
					},
				},
			},
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handler.Validate(ctx, logger, tt.request, "", now)
			assert.Equal(t, tt.wantAllow, resp.Allowed)
		})
	}
}
