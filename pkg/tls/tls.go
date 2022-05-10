package tls

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// CertRenewalInterval is the renewal interval for rootCA
const CertRenewalInterval time.Duration = 12 * time.Hour

// CertValidityDuration is the valid duration for a new cert
const CertValidityDuration time.Duration = 365 * 24 * time.Hour

//TlsCertificateGetExpirationDate Gets NotAfter property from raw certificate
func tlsCertificateGetExpirationDate(certData []byte) (*time.Time, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, errors.New("failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("failed to parse certificate: %v" + err.Error())
	}
	return &cert.NotAfter, nil
}
