package cosign

import (
	"context"

	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
)

// Deprecated: This package is deprecated. Use pkg/imageverification/imageverifiers/cosign instead.
// This file now delegates to the new implementation via an adapter for backward compatibility.

// NewVerifier creates a verifier that checks the UnifiedImageVerifiers feature flag.
// When enabled (default), it delegates to the new unified image verifier implementation.
// When disabled, it uses the legacy implementation for backward compatibility.
//
// Deprecated: Use pkg/imageverification/imageverifiers/cosign.NewVerifier instead.
// For ClusterPolicy, this function continues to work via the adapter layer.
// For new code, prefer ImageValidatingPolicy which uses the new verifier directly.
func NewVerifier() images.ImageVerifier {
	// Check feature flag to determine which implementation to use
	if toggle.FromContext(context.TODO()).UnifiedImageVerifiers() {
		logging.WithName("Cosign").V(4).Info("Using unified image verifier (new implementation)")
		return newClusterPolicyAdapter()
	}

	logging.WithName("Cosign").Info("Using legacy cosign implementation (feature flag disabled)")
	return newLegacyVerifier()
}

// newClusterPolicyAdapter creates the adapter - keeping it internal
func newClusterPolicyAdapter() images.ImageVerifier {
	adapter := &ClusterPolicyAdapter{}
	return adapter.init()
}

// VerifySignature is kept for backward compatibility but checks the feature flag.
// When UnifiedImageVerifiers is enabled (default), it delegates to the new adapter.
// When disabled, it uses the legacy implementation.
//
// Deprecated: Use pkg/imageverification/imageverifiers/cosign.Verifier.VerifyImageSignature instead.
// This function checks the UnifiedImageVerifiers feature flag to determine implementation.
func VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	verifier := NewVerifier()
	return verifier.VerifySignature(ctx, opts)
}

// FetchAttestations is kept for backward compatibility but checks the feature flag.
// When UnifiedImageVerifiers is enabled (default), it delegates to the new adapter.
// When disabled, it uses the legacy implementation.
//
// Deprecated: Use pkg/imageverification/imageverifiers/cosign.Verifier.VerifyAttestationSignature instead.
// This function checks the UnifiedImageVerifiers feature flag to determine implementation.
func FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	verifier := NewVerifier()
	return verifier.FetchAttestations(ctx, opts)
}
