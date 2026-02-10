// Package cosign provides image signature verification using Cosign.
//
// # Deprecation Notice
//
// This package is deprecated and maintained only for backward compatibility with ClusterPolicy.
// It delegates all functionality to pkg/imageverification/imageverifiers/cosign via an adapter layer.
//
// # Migration Guide
//
// For new code, use one of the following approaches:
//
//  1. **Recommended**: Use ImageValidatingPolicy instead of ClusterPolicy for image verification.
//     ImageValidatingPolicy uses pkg/imageverification/imageverifiers/cosign directly.
//
//  2. **Alternative**: If you must use ClusterPolicy, no changes are needed - this package
//     will continue to work via the adapter. However, be aware that:
//     - Transparency log verification is disabled by default for key-based verification
//     - Bundle format (Cosign v3) is fully supported
//     - TrustedMaterial is fetched from TUF on every verification
//
// # What Changed
//
// The refactoring unified the verification logic between ClusterPolicy and ImageValidatingPolicy:
//
//   - Old: pkg/cosign (ClusterPolicy only)
//   - New: pkg/imageverification/imageverifiers/cosign (both policies)
//   - Adapter: pkg/cosign/adapter.go (maintains backward compatibility)
//
// # Example
//
// If you were directly importing this package:
//
//	// Old (deprecated, but still works)
//	import "github.com/kyverno/kyverno/pkg/cosign"
//	verifier := cosign.NewVerifier()
//
//	// New (recommended)
//	import "github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/cosign"
//	verifier := cosign.NewVerifier(secretInterface, logger)
//
// # Removal Timeline
//
// This package will be maintained for backward compatibility through at least one major version.
// Direct usage outside of ClusterPolicy verification is discouraged and may be removed in a future release.
//
// See also: pkg/imageverification/imageverifiers/cosign
package cosign
