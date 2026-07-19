// Package sigstoretuf provides a single, process-wide synchronization point
// for every Kyverno code path that reads from or (re)initializes the shared
// sigstore TUF client singleton (github.com/sigstore/sigstore/pkg/tuf).
//
// # Background
//
// The sigstore/tuf package keeps one process-wide singleton TUF client and
// guards its own initialization routine (tuf.Initialize / tuf.NewFromEnv)
// with an internal package-level mutex. However, once the singleton exists,
// operations that refresh its cached metadata/targets (triggered from
// Initialize/NewFromEnv when the local cache is missing or expired) mutate
// internal maps on the *TUF value without holding *that* value's own mutex,
// while read paths such as GetTarget/GetTargetsByMeta *do* take the
// instance-level lock. Because these are two independent locks, concurrent
// calls into the TUF client from different goroutines can still race on the
// same underlying maps, producing a `fatal error: concurrent map writes`
// panic that crashes the whole process (see
// https://github.com/kyverno/kyverno/issues/15983).
//
// Every helper in cosign/sigstore-go that ends up touching the shared TUF
// singleton -- cosign.GetRekorPubs, cosign.GetCTLogPubs,
// fulcioroots.Get/GetIntermediates, and fetching "trusted_root.json" -- is
// susceptible to the same race, and is used by both the ImageValidatingPolicy
// (IVPOL) and ClusterPolicy (CPOL) cosign verifiers, as well as the one-shot
// TUF bootstrap performed by the admission/background/reports controllers on
// startup.
//
// Rather than re-implementing ad hoc, per-package mutexes (which only
// protect callers within that one package), this package centralizes the
// lock so IVPOL, CPOL and the CLI/controller bootstrap code all serialize
// through the same critical section. Callers should keep the locked region
// as small as possible: only the actual sigstore/cosign calls need to be
// inside the lock, not unrelated I/O such as registry pulls or Rekor client
// construction.
package sigstoretuf

import (
	"context"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"github.com/sigstore/sigstore/pkg/tuf"
)

// mu serializes every access to the shared sigstore TUF singleton across the
// whole process (IVPOL, CPOL and CLI/controller bootstrap).
var mu sync.Mutex

// Initialize (re)initializes the shared sigstore TUF client with the given
// mirror and trusted root, forcing a refresh of local metadata/targets. It
// must be used instead of calling tuf.Initialize directly so that concurrent
// initializations (e.g. triggered by concurrent IVPOL/CPOL verifications)
// cannot race with each other or with TrustedRoot/RekorPublicKeys/
// CTLogPublicKeys/FulcioRoots below.
func Initialize(ctx context.Context, mirror string, rootBytes []byte) error {
	mu.Lock()
	defer mu.Unlock()
	return tuf.Initialize(ctx, mirror, rootBytes)
}

// TrustedRoot returns the "trusted_root.json" target from the shared TUF
// client, initializing it from the local cache/environment if needed.
func TrustedRoot(ctx context.Context) (*root.TrustedRoot, error) {
	mu.Lock()
	defer mu.Unlock()

	tufClient, err := tuf.NewFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("initializing tuf: %w", err)
	}
	targetBytes, err := tufClient.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("error getting target trusted_root.json: %w", err)
	}
	trustedRoot, err := root.NewTrustedRootFromJSON(targetBytes)
	if err != nil {
		return nil, fmt.Errorf("error creating trusted root: %w", err)
	}
	return trustedRoot, nil
}

// RekorPublicKeys returns the trusted Rekor public keys from the shared TUF
// client (or the alternate key configured via the
// SIGSTORE_REKOR_PUBLIC_KEY env var, see cosign.GetRekorPubs).
func RekorPublicKeys(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	mu.Lock()
	defer mu.Unlock()
	return cosign.GetRekorPubs(ctx)
}

// CTLogPublicKeys returns the trusted CTLog public keys from the shared TUF
// client (see cosign.GetCTLogPubs).
func CTLogPublicKeys(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	mu.Lock()
	defer mu.Unlock()
	return cosign.GetCTLogPubs(ctx)
}

// FulcioRoots returns the Fulcio root and intermediate certificate pools
// sourced from the shared TUF client (see fulcioroots.Get/GetIntermediates).
func FulcioRoots() (*x509.CertPool, *x509.CertPool, error) {
	mu.Lock()
	defer mu.Unlock()
	roots, err := fulcioroots.Get()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Fulcio roots: %w", err)
	}
	intermediates, err := fulcioroots.GetIntermediates()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Fulcio intermediates: %w", err)
	}
	return roots, intermediates, nil
}
