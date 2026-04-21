package cosign

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repeatPEMBlock concatenates the given PEM block count times. This lets tests
// build arbitrarily long chains without paying RSA key-generation cost per cert.
func repeatPEMBlock(block []byte, count int) []byte {
	var buf bytes.Buffer
	buf.Grow(len(block) * count)
	for i := 0; i < count; i++ {
		buf.Write(block)
	}
	return buf.Bytes()
}

// buildTSAChainPEM returns a PEM containing one self-signed root CA followed by
// numIntermediates copies of a single intermediate CA PEM block. Repeating the same
// intermediate (rather than regenerating fresh RSA keys per cert) keeps tests fast
// while still producing numIntermediates distinct intermediate blocks for limit checks.
// Helpers generateRootCA, generateIntermediateCA, and certToPEM are defined in
// certs_test.go (same package).
func buildTSAChainPEM(t *testing.T, numIntermediates int) string {
	t.Helper()
	root, rootKey := generateRootCA(t)
	intermediate, _ := generateIntermediateCA(t, root, rootKey)
	var buf bytes.Buffer
	buf.Write(certToPEM(root))
	buf.Write(repeatPEMBlock(certToPEM(intermediate), numIntermediates))
	return buf.String()
}

// buildMultiCertPEM returns a PEM containing numCerts copies of a single self-signed
// CA certificate — suitable for testing chain-length limits since cryptoutils.LoadCertificatesFromPEM
// returns all certs without checking chain structure. Reusing one cert avoids the
// cost of generating a fresh RSA key per block.
func buildMultiCertPEM(t *testing.T, numCerts int) string {
	t.Helper()
	cert, _ := generateRootCA(t)
	return string(repeatPEMBlock(certToPEM(cert), numCerts))
}

// TestCheckOptions_TSACertChainTooManyIntermediates verifies that a TSA chain with more
// than maxIntermediateCerts intermediate CAs is rejected.
func TestCheckOptions_TSACertChainTooManyIntermediates(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	tsaChain := buildTSAChainPEM(t, maxIntermediateCerts+1)

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
			InsecureIgnoreSCT:  true,
			TSACertChain:       tsaChain,
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TSA certificate chain contains too many")
}

// TestCheckOptions_TSACertChainAtLimit verifies that a TSA chain with exactly
// maxIntermediateCerts intermediates is not rejected by the count limit.
func TestCheckOptions_TSACertChainAtLimit(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	tsaChain := buildTSAChainPEM(t, maxIntermediateCerts)

	cosignCfg := &v1beta1.Cosign{
		Key: &v1beta1.Key{
			Data: testPublicKey,
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
			InsecureIgnoreSCT:  true,
			TSACertChain:       tsaChain,
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	if err != nil {
		assert.False(t, strings.Contains(err.Error(), "TSA certificate chain contains too many"),
			"boundary value (%d intermediates) must not be rejected by the count check; got: %v", maxIntermediateCerts, err)
	}
}

// TestCheckOptions_CertChainTooLong verifies that att.Certificate.CertificateChain with
// more than maxIntermediateCerts+1 entries is rejected.
func TestCheckOptions_CertChainTooLong(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	// Leaf cert for att.Certificate.Certificate.
	leafCert, _ := generateRootCA(t)
	leafPEM := string(certToPEM(leafCert))

	chainPEM := buildMultiCertPEM(t, maxIntermediateCerts+2)

	cosignCfg := &v1beta1.Cosign{
		Certificate: &v1beta1.Certificate{
			Certificate:      &v1beta1.StringOrExpression{Value: leafPEM},
			CertificateChain: &v1beta1.StringOrExpression{Value: chainPEM},
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
			InsecureIgnoreSCT:  true,
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate chain too long")
}

// TestCheckOptions_CertChainAtLimit verifies that a CertificateChain with exactly
// maxIntermediateCerts+1 entries is not rejected by the length check.
func TestCheckOptions_CertChainAtLimit(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	leafCert, _ := generateRootCA(t)
	leafPEM := string(certToPEM(leafCert))

	chainPEM := buildMultiCertPEM(t, maxIntermediateCerts+1)

	cosignCfg := &v1beta1.Cosign{
		Certificate: &v1beta1.Certificate{
			Certificate:      &v1beta1.StringOrExpression{Value: leafPEM},
			CertificateChain: &v1beta1.StringOrExpression{Value: chainPEM},
		},
		CTLog: &v1beta1.CTLog{
			URL:                "https://rekor.sigstore.dev",
			InsecureIgnoreTlog: true,
			InsecureIgnoreSCT:  true,
		},
	}

	_, err := checkOptions(ctx, cosignCfg, baseROpts, baseNOpts, nil)
	// May fail for chain-validation reasons but must not fail on the length check.
	if err != nil {
		assert.False(t, strings.Contains(err.Error(), "certificate chain too long"),
			"boundary chain (%d certs) must not be rejected by the length check; got: %v", maxIntermediateCerts+1, err)
	}
}
