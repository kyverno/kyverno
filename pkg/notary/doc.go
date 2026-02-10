// Package notary provides image signature verification using Notary.
//
// # Deprecation Notice
//
// This package is deprecated and maintained only for backward compatibility with ClusterPolicy.
// It delegates all functionality to pkg/imageverification/imageverifiers/notary via an adapter layer.
//
// # Migration Guide
//
// For new code, use one of the following approaches:
//
//  1. **Recommended**: Use ImageValidatingPolicy instead of ClusterPolicy for image verification.
//     ImageValidatingPolicy uses pkg/imageverification/imageverifiers/notary directly.
//
//  2. **Alternative**: If you must use ClusterPolicy, no changes are needed - this package
//     will continue to work via the adapter. However, be aware that:
//     - Bundle format (Cosign v3) is fully supported
//     - Authentication may be limited for private registries
//     - Certificate validation follows the new verifier architecture
//
// # What Changed
//
// The refactoring unified the verification logic between ClusterPolicy and ImageValidatingPolicy:
//
//   - Old: pkg/notary (ClusterPolicy only)
//   - New: pkg/imageverification/imageverifiers/notary (both policies)
//   - Adapter: pkg/notary/adapter.go (maintains backward compatibility)
//
// # Example
//
// If you were directly importing this package:
//
//	// Old (deprecated, but still works)
//	import "github.com/kyverno/kyverno/pkg/notary"
//	verifier := notary.NewVerifier()
//
//	// New (recommended)
//	import "github.com/kyverno/kyverno/pkg/imageverification/imageverifiers/notary"
//	verifier := notary.NewVerifier(logger)
//
// # Removal Timeline
//
// This package will be maintained for backward compatibility through at least one major version.
// Direct usage outside of ClusterPolicy verification is discouraged and may be removed in a future release.
//
// See also: pkg/imageverification/imageverifiers/notary
package notary
