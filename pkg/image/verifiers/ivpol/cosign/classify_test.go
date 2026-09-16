package cosign

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/assert"
)

func TestIsNoBundle(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"no matching attestations (empty referrers index)", &cosign.ErrNoMatchingAttestations{}, true},
		{"image tag not found", &cosign.ErrImageTagNotFound{}, true},
		{"wrapped no matching attestations", fmt.Errorf("detect: %w", &cosign.ErrNoMatchingAttestations{}), true},
		{"transport/infra error", errors.New("dial tcp: connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNoBundle(tt.err))
		})
	}
}

func TestIsInfraError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"registry HTTP error", &transport.Error{StatusCode: 403}, true},
		{"tls unknown authority", x509.UnknownAuthorityError{}, true},
		{"url error", &url.Error{Op: "Get", URL: "https://ghcr.io", Err: errors.New("boom")}, true},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped registry error", fmt.Errorf("fetch: %w", &transport.Error{StatusCode: 500}), true},
		{"genuine verification failure", &cosign.ErrNoMatchingSignatures{}, false},
		{"generic verification failure", &cosign.VerificationFailure{}, false},
		{"unknown/opaque error", errors.New("something went wrong"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isInfraError(tt.err))
		})
	}
}

// Only a recognizable infra failure becomes a SetupError; genuine and
// unrecognized errors must fail closed.
func TestClassifyVerifyError(t *testing.T) {
	genuine := classifyVerifyError(&cosign.ErrNoMatchingSignatures{})
	assert.False(t, IsSetup(genuine), "genuine verification failure must not be a SetupError")

	unknown := classifyVerifyError(errors.New("some future cosign error we don't recognize"))
	assert.False(t, IsSetup(unknown), "unknown errors must fail closed, not be treated as setup errors")

	infra := classifyVerifyError(&transport.Error{StatusCode: 403})
	assert.True(t, IsSetup(infra), "infrastructure failure must be a SetupError")
}
