package notary

import (
	"context"

	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
)

// Deprecated: This package is deprecated. Use pkg/imageverification/imageverifiers/notary instead.
// This file now delegates to the new implementation via an adapter for backward compatibility.

// Constants used by the old notary implementation files that still exist
var (
	maxReferrersCount = 50
	maxPayloadSize    = int64(10 * 1000 * 1000) // 10 MB
)

// NewVerifier creates a verifier that checks the UnifiedImageVerifiers feature flag.
// When enabled (default), it delegates to the new unified image verifier implementation.
// When disabled, it uses the legacy implementation for backward compatibility.
//
// Deprecated: Use pkg/imageverification/imageverifiers/notary.NewVerifier instead.
// For ClusterPolicy, this function continues to work via the adapter layer.
// For new code, prefer ImageValidatingPolicy which uses the new verifier directly.
func NewVerifier() images.ImageVerifier {
	// Check feature flag to determine which implementation to use
	if toggle.FromContext(context.TODO()).UnifiedImageVerifiers() {
		logging.WithName("Notary").V(4).Info("Using unified image verifier (new implementation)")
		return newClusterPolicyAdapter()
	}

	logging.WithName("Notary").Info("Using legacy notary implementation (feature flag disabled)")
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
// Deprecated: Use pkg/imageverification/imageverifiers/notary.Verifier.VerifyImageSignature instead.
// This function checks the UnifiedImageVerifiers feature flag to determine implementation.
func VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	verifier := NewVerifier()
	return verifier.VerifySignature(ctx, opts)
}

// FetchAttestations is kept for backward compatibility but checks the feature flag.
// When UnifiedImageVerifiers is enabled (default), it delegates to the new adapter.
// When disabled, it uses the legacy implementation.
//
// Deprecated: Use pkg/imageverification/imageverifiers/notary.Verifier.VerifyAttestationSignature instead.
// This function checks the UnifiedImageVerifiers feature flag to determine implementation.
func FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	verifier := NewVerifier()
	return verifier.FetchAttestations(ctx, opts)
}
