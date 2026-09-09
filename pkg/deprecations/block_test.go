package deprecations

import (
	"context"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// resetBlockLegacyPolicyAPIs returns the shared BlockLegacyPolicyAPIs toggle to its unset state
// (default: enabled) once the test finishes, so tests that flip it don't leak into others.
func resetBlockLegacyPolicyAPIs(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse(""))
	})
}

func neverCalled(t *testing.T) func() bool {
	t.Helper()
	return func() bool {
		t.Fatal("specsEqual should not have been invoked")
		return false
	}
}

func TestShouldBlock(t *testing.T) {
	clusterPolicyKind := metav1.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}
	celPolicyExceptionKind := metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1", Kind: "PolicyException"}
	validatingPolicyKind := metav1.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1beta1", Kind: "ValidatingPolicy"}

	tests := []struct {
		name       string
		toggleOff  bool
		request    admissionv1.AdmissionRequest
		specsEqual func() bool
		wantBlock  bool
	}{
		{
			name:      "toggle disabled allows create of a legacy kind",
			toggleOff: true,
			request:   admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Create},
			wantBlock: false,
		},
		{
			name:      "status subresource is never blocked",
			request:   admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Update, SubResource: "status"},
			wantBlock: false,
		},
		{
			name:      "delete is never blocked",
			request:   admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Delete},
			wantBlock: false,
		},
		{
			name:      "connect is never blocked",
			request:   admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Connect},
			wantBlock: false,
		},
		{
			name:      "CEL PolicyException (policies.kyverno.io) is never blocked",
			request:   admissionv1.AdmissionRequest{Kind: celPolicyExceptionKind, Operation: admissionv1.Create},
			wantBlock: false,
		},
		{
			name:      "non-legacy kind is never blocked",
			request:   admissionv1.AdmissionRequest{Kind: validatingPolicyKind, Operation: admissionv1.Create},
			wantBlock: false,
		},
		{
			name:      "create of a legacy kind is blocked",
			request:   admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Create},
			wantBlock: true,
		},
		{
			name:       "no-op update (identical spec) is allowed",
			request:    admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Update},
			specsEqual: func() bool { return true },
			wantBlock:  false,
		},
		{
			name:       "spec-changing update is blocked",
			request:    admissionv1.AdmissionRequest{Kind: clusterPolicyKind, Operation: admissionv1.Update},
			specsEqual: func() bool { return false },
			wantBlock:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetBlockLegacyPolicyAPIs(t)
			if tt.toggleOff {
				require.NoError(t, toggle.BlockLegacyPolicyAPIs.Parse("false"))
			}
			specsEqual := tt.specsEqual
			if specsEqual == nil {
				specsEqual = neverCalled(t)
			}

			err, blocked := ShouldBlock(context.Background(), tt.request, specsEqual)
			assert.Equal(t, tt.wantBlock, blocked)
			if tt.wantBlock {
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), "no longer accepted for create or update"))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
