package cosign

import (
	"context"

	"github.com/kyverno/kyverno/pkg/images"
)

// Deprecated: This package is deprecated. Use pkg/imageverification/imageverifiers/cosign instead.
// This file now delegates to the new implementation via an adapter for backward compatibility.

// NewVerifier creates a verifier that delegates to the new cosign verifier implementation.
// This maintains backward compatibility with ClusterPolicy image verification.
func NewVerifier() images.ImageVerifier {
	// Delegate to the adapter which wraps the new verifier
	return newClusterPolicyAdapter()
}

// newClusterPolicyAdapter creates the adapter - keeping it internal
func newClusterPolicyAdapter() images.ImageVerifier {
	adapter := &ClusterPolicyAdapter{}
	return adapter.init()
}

// VerifySignature is kept for backward compatibility but delegates to the adapter
func VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	verifier := NewVerifier()
	return verifier.VerifySignature(ctx, opts)
}

// FetchAttestations is kept for backward compatibility but delegates to the adapter
func FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	verifier := NewVerifier()
	return verifier.FetchAttestations(ctx, opts)
}
