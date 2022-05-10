package tls

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
)

// PrivateKeyToPem Creates PEM block from private key object
func PrivateKeyToPem(rsaKey *rsa.PrivateKey) []byte {
	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}
	return pem.EncodeToMemory(privateKey)
}

// CertificateToPem Creates PEM block from certificate object
func CertificateToPem(cert *x509.Certificate) []byte {
	certificate := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(certificate)
}
