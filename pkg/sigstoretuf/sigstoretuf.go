// Package sigstoretuf preserves the shared trust helpers used by existing
// verifiers. Repository access uses sigstore-go through sigstoreguard, including
// its guarded HTTP client and process-wide cache synchronization.
package sigstoretuf

import (
	"context"
	"crypto/x509"

	"github.com/kyverno/kyverno/pkg/sigstoreguard"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
)

// Initialize refreshes the operator repository before publishing its defaults.
func Initialize(ctx context.Context, mirror string, rootBytes []byte) error {
	return sigstoreguard.Initialize(ctx, mirror, rootBytes)
}

// TrustedRoot returns trusted_root.json from the operator repository.
func TrustedRoot(ctx context.Context) (*root.TrustedRoot, error) {
	return sigstoreguard.TrustedRoot(ctx)
}

// RekorPublicKeys preserves key-file overrides and legacy PEM target support.
func RekorPublicKeys(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	return sigstoreguard.RekorPublicKeys(ctx)
}

// CTLogPublicKeys preserves key-file overrides and legacy PEM target support.
func CTLogPublicKeys(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	return sigstoreguard.CTLogPublicKeys(ctx)
}

// FulcioRoots returns the operator's Fulcio root and intermediate certificates.
func FulcioRoots() (*x509.CertPool, *x509.CertPool, error) {
	return FulcioRootsWithContext(context.Background())
}

// FulcioRootsWithContext also honors cancellation while loading trust material.
func FulcioRootsWithContext(ctx context.Context) (*x509.CertPool, *x509.CertPool, error) {
	return sigstoreguard.FulcioRootsWithContext(ctx)
}

// WithLock serializes cache operations. The callback must not call another
// helper that acquires the same non-reentrant cache lock.
func WithLock(fn func() error) error {
	return sigstoreguard.WithLock(fn)
}
