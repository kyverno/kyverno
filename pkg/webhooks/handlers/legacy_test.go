package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
)

// TestWithLegacyPolicyDenialChainPlacement proves what this package is responsible for: chain
// placement, not status/finalizer semantics (those belong to pkg/deprecations, which this
// package does not import a registrant of). A status and a scale subresource request must both
// reach decide, which they would not if the decorator were chained inside
// WithSubResourceFilter().
func TestWithLegacyPolicyDenialChainPlacement(t *testing.T) {
	tests := []struct {
		name           string
		subResource    string
		decideErr      error
		decideDeny     bool
		decideRecovery bool
	}{
		{name: "top-level request reaches decide and is denied", decideErr: errors.New("denied"), decideDeny: true},
		{name: "top-level request reaches decide and is allowed", decideDeny: false},
		{name: "status subresource reaches decide before subresource filter can swallow it", subResource: "status", decideErr: errors.New("denied"), decideDeny: true},
		{name: "scale subresource reaches decide before subresource filter can swallow it", subResource: "scale", decideErr: errors.New("denied"), decideDeny: true},
		{name: "finalizer recovery short-circuits without reaching the typed inner handler", decideRecovery: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var decideCalled, innerCalled bool
			decide := func(request admissionv1.AdmissionRequest) (deprecations.Decision, error) {
				decideCalled = true
				assert.Equal(t, tt.subResource, request.SubResource)
				if tt.decideDeny {
					return deprecations.Deny, tt.decideErr
				}
				if tt.decideRecovery {
					return deprecations.AllowRecovery, nil
				}
				return deprecations.Delegate, nil
			}
			inner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
				innerCalled = true
				return AdmissionResponse{Allowed: true}
			})

			// Chain exactly as server.go does: WithSubResourceFilter() first (innermost to the
			// decorator, i.e. between it and the real inner handler), then
			// withLegacyPolicyDenial() outside it, so a subresource request must still reach
			// decide.
			handler := inner.WithSubResourceFilter().withLegacyPolicyDenial(decide)

			request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{SubResource: tt.subResource}}
			response := handler(context.Background(), logr.Discard(), request, time.Now())

			assert.True(t, decideCalled, "decide must be called regardless of subresource")
			switch {
			case tt.decideDeny:
				assert.False(t, innerCalled, "inner must not be called when decide denies")
				assert.False(t, response.Allowed)
			case tt.decideRecovery:
				// The inner handler is the typed legacy validator; reaching it would let
				// stale-spec validation refuse a recovery the gate already permitted.
				assert.False(t, innerCalled, "inner must not be called for finalizer recovery")
				assert.True(t, response.Allowed)
			default:
				assert.True(t, innerCalled, "inner must be called when decide delegates")
			}
		})
	}
}
