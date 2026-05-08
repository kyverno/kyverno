// This file contains end-to-end tests for the sigstore-go-direct bundle
// verification path, exercised against a real GitHub Actions Artifact
// Attestations bundle + TSA cert chain.
//
// Tests are skipped when the fixtures aren't present, so this file is
// safe to keep in the upstream tree even though the fixtures live outside
// the repository (in /home/jim/dev/3pp/kyverno-tsa-debug). See
// sigstore/cosign#4847 for the bug these fixtures reproduce.

package cosign

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadE2EFixtures loads bundle.json + tsaCertChain.pem from the directory
// pointed to by the KYVERNO_TSA_FIXTURE_DIR env var. The reproducer at
// github.com/jimgus/kyverno-tsa-debug produces these artefacts from a real
// GitHub Actions Artifact Attestation on a private repo. CI / contributors
// without the fixture skip transparently — this keeps the test safe to ship
// upstream without bundling the actual artefacts.
func loadE2EFixtures(t *testing.T) (bundleJSON, chainPEM []byte) {
	t.Helper()
	dir := os.Getenv("KYVERNO_TSA_FIXTURE_DIR")
	if dir == "" {
		t.Skip("KYVERNO_TSA_FIXTURE_DIR not set; skipping GitHub-TSA e2e fixture test (set to a directory containing bundle.json + tsaCertChain.pem produced by github.com/jimgus/kyverno-tsa-debug)")
	}
	bundlePath := filepath.Join(dir, "bundle.json")
	chainPath := filepath.Join(dir, "tsaCertChain.pem")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Skipf("e2e fixture %s not available: %v", bundlePath, err)
	}
	if _, err := os.Stat(chainPath); err != nil {
		t.Skipf("e2e fixture %s not available: %v", chainPath, err)
	}
	var err error
	bundleJSON, err = os.ReadFile(bundlePath)
	require.NoError(t, err)
	chainPEM, err = os.ReadFile(chainPath)
	require.NoError(t, err)
	return bundleJSON, chainPEM
}

func splitChainE2E(t *testing.T, pemBytes []byte) (leaf *x509.Certificate, intermediates, roots []*x509.Certificate) {
	t.Helper()
	rest := pemBytes
	for {
		block, r := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = r
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		switch {
		case !c.IsCA:
			leaf = c
		case bytes.Equal(c.RawIssuer, c.RawSubject):
			roots = append(roots, c)
		default:
			intermediates = append(intermediates, c)
		}
	}
	return
}

// TestE2E_TimestampVerificationFailsWithoutCustomTSA confirms the bug is
// real on this fixture: with TrustedMaterial that doesn't include the
// GitHub TSA, sigstore-go's signed-timestamp verification fails.
func TestE2E_TimestampVerificationFailsWithoutCustomTSA(t *testing.T) {
	bundleJSON, _ := loadE2EFixtures(t)

	var b sgbundle.Bundle
	require.NoError(t, b.UnmarshalJSON(bundleJSON))

	publicRoot := emptyPublicTrustedRoot(t)

	// Pass nil for the custom TSA so composeTrustedMaterial returns the
	// public root unchanged. This simulates the unfixed Kyverno IVPOL flow.
	tm, err := composeTrustedMaterial(publicRoot, nil, nil, nil)
	require.NoError(t, err)

	_, err = verify.VerifySignedTimestampWithThreshold(&b, tm, 1)
	assert.Error(t, err, "without the custom TSA in TrustedMaterial, sigstore-go must fail to verify the GitHub-TSA-signed timestamp")
}

// TestE2E_TimestampVerificationSucceedsWithCustomTSA is the fix-confirms-the-
// fix test. With the same fixture and an empty public root, but with the
// caller-provided TSA cert chain composed into the TrustedMaterial,
// sigstore-go's signed-timestamp verification succeeds.
func TestE2E_TimestampVerificationSucceedsWithCustomTSA(t *testing.T) {
	bundleJSON, chainPEM := loadE2EFixtures(t)

	var b sgbundle.Bundle
	require.NoError(t, b.UnmarshalJSON(bundleJSON))

	leaf, intermediates, roots := splitChainE2E(t, chainPEM)
	require.NotNil(t, leaf, "fixture must contain a leaf cert")
	require.NotEmpty(t, roots, "fixture must contain at least one root cert")

	publicRoot := emptyPublicTrustedRoot(t)

	tm, err := composeTrustedMaterial(publicRoot, leaf, intermediates, roots)
	require.NoError(t, err)

	verifiedTimestamps, err := verify.VerifySignedTimestampWithThreshold(&b, tm, 1)
	assert.NoError(t, err, "with the GitHub TSA composed into TrustedMaterial, sigstore-go must verify the timestamp")
	assert.NotEmpty(t, verifiedTimestamps, "expected at least one verified timestamp")
}
