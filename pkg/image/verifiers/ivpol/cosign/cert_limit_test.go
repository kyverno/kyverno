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

// buildTSAChainPEM returns a PEM containing one self-signed root CA followed by
// numIntermediates intermediate CA certificates, all signed by the same root.
// Helper functions generateRootCA, generateIntermediateCA, and certToPEM are defined
// in certs_test.go (same package).
func buildTSAChainPEM(t *testing.T, numIntermediates int) string {
	t.Helper()
	root, rootKey := generateRootCA(t)
	var buf bytes.Buffer
	buf.Write(certToPEM(root))
	for i := 0; i < numIntermediates; i++ {
		intermediate, _ := generateIntermediateCA(t, root, rootKey)
		buf.Write(certToPEM(intermediate))
	}
	return buf.String()
}

// buildMultiCertPEM returns a PEM containing numCerts self-signed CA certificates
// concatenated — suitable for testing chain-length limits since cryptoutils.LoadCertificatesFromPEM
// returns all certs without checking chain structure.
func buildMultiCertPEM(t *testing.T, numCerts int) string {
	t.Helper()
	var buf bytes.Buffer
	for i := 0; i < numCerts; i++ {
		cert, _ := generateRootCA(t)
		buf.Write(certToPEM(cert))
	}
	return buf.String()
}

// TestCheckOptions_TSACertChainTooManyIntermediates verifies that a TSA chain with more
// than maxIntermediateCerts (10) intermediate CAs is rejected.
func TestCheckOptions_TSACertChainTooManyIntermediates(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	// 11 intermediates — exceeds maxIntermediateCerts=10.
	tsaChain := buildTSAChainPEM(t, 11)

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
	assert.Contains(t, err.Error(), "TSA certificate chain contains too many intermediate certificates")
}

// TestCheckOptions_TSACertChainAtLimit verifies that a TSA chain with exactly
// maxIntermediateCerts (10) intermediates is not rejected by the count limit.
func TestCheckOptions_TSACertChainAtLimit(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	// Exactly 10 intermediates — at the boundary, must pass the count check.
	tsaChain := buildTSAChainPEM(t, 10)

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
		assert.False(t, strings.Contains(err.Error(), "TSA certificate chain contains too many intermediate certificates"),
			"boundary value (10 intermediates) must not be rejected by the count check; got: %v", err)
	}
}

// TestCheckOptions_CertChainTooLong verifies that att.Certificate.CertificateChain with
// more than maxIntermediateCerts+1 (11) entries is rejected.
func TestCheckOptions_CertChainTooLong(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	// Leaf cert for att.Certificate.Certificate.
	leafCert, _ := generateRootCA(t)
	leafPEM := string(certToPEM(leafCert))

	// 12 certs in the chain — exceeds maxIntermediateCerts+1=11.
	chainPEM := buildMultiCertPEM(t, 12)

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
// maxIntermediateCerts+1 (11) entries is not rejected by the length check.
func TestCheckOptions_CertChainAtLimit(t *testing.T) {
	ctx := context.TODO()
	baseROpts, baseNOpts := baseOpts()

	leafCert, _ := generateRootCA(t)
	leafPEM := string(certToPEM(leafCert))

	// Exactly 11 certs in the chain — at the boundary.
	chainPEM := buildMultiCertPEM(t, 11)

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
			"boundary chain (11 certs) must not be rejected by the length check; got: %v", err)
	}
}
