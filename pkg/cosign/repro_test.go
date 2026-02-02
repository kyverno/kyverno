package cosign

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func createCert(t *testing.T, template *x509.Certificate, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (*x509.Certificate, []byte, interface{}) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	pubKey := &priv.PublicKey
	if pub != nil {
		pubKey = pub.(*rsa.PublicKey)
	}

	signerPriv := priv
	if parentPriv != nil {
		signerPriv = parentPriv.(*rsa.PrivateKey)
	}
	
	if parent == nil {
		parent = template
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, pubKey, signerPriv)
	assert.NoError(t, err)

	cert, err := x509.ParseCertificate(certBytes)
	assert.NoError(t, err)

	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	return cert, pemBlock, priv
}

func TestSplitPEMCertificateChain_DigiCertStructure(t *testing.T) {
	// Root CA (Self-signed)
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Root CA"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	rootCert, rootPEM, rootPriv := createCert(t, rootTemplate, nil, nil, nil)

	// Cross Root CA (Signed by Root, No EKU)
	crossTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Cross Root CA"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		// Explicitly NO EKU
		ExtKeyUsage: nil,
	}
	crossCert, crossPEM, crossPriv := createCert(t, crossTemplate, rootCert, nil, rootPriv)

	// Intermediate CA (Signed by Cross Root, Has Timestamping EKU)
	interTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "Intermediate CA"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
	}
	interCert, interPEM, interPriv := createCert(t, interTemplate, crossCert, nil, crossPriv)

	// Leaf TSA (Signed by Intermediate, Has Timestamping EKU)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject:      pkix.Name{CommonName: "Leaf TSA"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		IsCA:         false,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
	}
	_, leafPEM, _ := createCert(t, leafTemplate, interCert, nil, interPriv)

	// Chain: Leaf -> Intermediate -> CrossRoot -> Root
	fullChain := bytes.Join([][]byte{leafPEM, interPEM, crossPEM, rootPEM}, []byte("\n"))

	leaves, intermediates, roots, err := splitPEMCertificateChain(fullChain)
	assert.NoError(t, err)

	// Check Leaves
	assert.Equal(t, 1, len(leaves))
	assert.Equal(t, "Leaf TSA", leaves[0].Subject.CommonName)

	// Check Roots (Only self-signed)
	assert.Equal(t, 1, len(roots))
	assert.Equal(t, "Root CA", roots[0].Subject.CommonName)

	// Check Intermediates
	// CURRENT BEHAVIOR: Cross Root is classified as intermediate because it's not self-signed
	assert.Equal(t, 2, len(intermediates))
	
	names := []string{intermediates[0].Subject.CommonName, intermediates[1].Subject.CommonName}
	assert.Contains(t, names, "Intermediate CA")
	assert.Contains(t, names, "Cross Root CA")
	
	// Verify that Cross Root CA (which had no EKU) now has TimeStamping EKU injected
	for _, cert := range intermediates {
		if cert.Subject.CommonName == "Cross Root CA" {
			assert.Equal(t, 1, len(cert.ExtKeyUsage))
			assert.Equal(t, x509.ExtKeyUsageTimeStamping, cert.ExtKeyUsage[0])
		}
	}
}
