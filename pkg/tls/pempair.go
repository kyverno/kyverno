package tls

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
)

// PemPair The pair of TLS certificate corresponding private key, both in PEM format
type PemPair struct {
	Certificate []byte
	PrivateKey  []byte
}

// PrivateKeyToPem Creates PEM block from private key object
func PrivateKeyToPem(rsaKey *rsa.PrivateKey) []byte {
	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}
	return pem.EncodeToMemory(privateKey)
}

// CertificateToPem ...
func CertificateToPem(certificateDER []byte) []byte {
	certificate := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	}
	return pem.EncodeToMemory(certificate)
}
