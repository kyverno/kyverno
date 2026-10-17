package internal

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	gcrname "github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
)

// mockRegistryClient is a minimal RegistryClient that propagates context errors.
type mockRegistryClient struct{}

func (m *mockRegistryClient) ForRef(ctx context.Context, ref string) (*engineapi.ImageData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRegistryClient) FetchImageDescriptor(ctx context.Context, ref string) (*gcrremote.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockRegistryClient) Keychain() authn.Keychain {
	return authn.DefaultKeychain
}

func (m *mockRegistryClient) Options(ctx context.Context) ([]gcrremote.Option, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (m *mockRegistryClient) NameOptions() []gcrname.Option {
	return nil
}

func newTestImageVerifier() *ImageVerifier {
	return &ImageVerifier{
		logger: logr.Discard(),
		rule:   kyvernov1.Rule{Name: "test-rule"},
	}
}

func newTestImageVerifierWithClient() *ImageVerifier {
	return &ImageVerifier{
		logger:  logr.Discard(),
		rclient: &mockRegistryClient{},
		rule:    kyvernov1.Rule{Name: "test-rule"},
	}
}

func TestHandleRegistryErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		err            error
		expectedStatus engineapi.RuleStatus
	}{
		{
			name:           "network error returns RuleError",
			err:            &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")},
			expectedStatus: engineapi.RuleStatusError,
		},
		{
			name:           "context canceled returns RuleError",
			err:            context.Canceled,
			expectedStatus: engineapi.RuleStatusError,
		},
		{
			name:           "context deadline exceeded returns RuleError",
			err:            context.DeadlineExceeded,
			expectedStatus: engineapi.RuleStatusError,
		},
		{
			name:           "wrapped context canceled returns RuleError",
			err:            fmt.Errorf("Get \"test-image:latest\": %w", context.Canceled),
			expectedStatus: engineapi.RuleStatusError,
		},
		{
			name:           "wrapped deadline exceeded returns RuleError",
			err:            fmt.Errorf("operation error: DescribeKey: %w", context.DeadlineExceeded),
			expectedStatus: engineapi.RuleStatusError,
		},
		{
			name:           "signature mismatch returns RuleFail",
			err:            fmt.Errorf("no matching signatures: signature mismatch"),
			expectedStatus: engineapi.RuleStatusFail,
		},
		{
			name:           "generic error returns RuleFail",
			err:            fmt.Errorf("failed to load public key"),
			expectedStatus: engineapi.RuleStatusFail,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			iv := newTestImageVerifier()
			resp := iv.handleRegistryErrors("test-image:latest", tt.err)
			if resp.Status() != tt.expectedStatus {
				t.Errorf("expected %s, got %s", tt.expectedStatus, resp.Status())
			}
		})
	}
}

func TestVerifyAttestors_InfraErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		makeCtx func() (context.Context, context.CancelFunc)
	}{
		{
			name: "canceled context returns RuleError",
			makeCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
		},
		{
			name: "expired deadline returns RuleError",
			makeCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 0)
			},
		},
	}
	attestors := []kyvernov1.AttestorSet{
		{
			Entries: []kyvernov1.Attestor{
				{
					Keys: &kyvernov1.StaticKeyAttestor{
						PublicKeys: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE0f0yKRJhhVLpwQ5GdEm2Wfg7mDc
sFoG1k2SHF5h8B4rkl46GnVHgzGPGMVBk3omHqo3KVajkSMSIw9+U6xPOA==
-----END PUBLIC KEY-----`,
						SignatureAlgorithm: "sha256",
					},
				},
			},
		},
	}
	imageVerify := kyvernov1.ImageVerification{
		ImageReferences: []string{"*"},
	}
	imageInfo := apiutils.ImageInfo{
		ImageInfo: imageutils.ImageInfo{
			Registry: "registry.example.com",
			Name:     "test-app",
			Path:     "test-app",
			Tag:      "latest",
		},
		Pointer: "/spec/containers/0/image",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			iv := newTestImageVerifierWithClient()
			ctx, cancel := tt.makeCtx()
			defer cancel()

			resp, cosignResp := iv.verifyAttestors(ctx, attestors, imageVerify, imageInfo)
			if cosignResp != nil {
				t.Errorf("expected nil cosign response, got %v", cosignResp)
			}
			if resp.Status() != engineapi.RuleStatusError {
				t.Errorf("expected RuleStatusError, got %s: %s", resp.Status(), resp.Message())
			}
		})
	}
}
