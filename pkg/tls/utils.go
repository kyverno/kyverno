package tls

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	corev1 "k8s.io/api/core/v1"
)

func privateKeyToPem(rsaKey *rsa.PrivateKey) []byte {
	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}
	return pem.EncodeToMemory(privateKey)
}

func certificateToPem(certs ...*x509.Certificate) []byte {
	var raw []byte
	for _, cert := range certs {
		certificate := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}
		raw = append(raw, pem.EncodeToMemory(certificate)...)
	}
	return raw
}

func pemToPrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func pemToCertificates(raw []byte) []*x509.Certificate {
	var certs []*x509.Certificate
	for {
		certPemBlock, next := pem.Decode(raw)
		if certPemBlock == nil {
			return certs
		}
		raw = next
		cert, err := x509.ParseCertificate(certPemBlock.Bytes)
		if err == nil {
			certs = append(certs, cert)
		}
	}
}

func removeExpiredCertificates(now time.Time, certs ...*x509.Certificate) []*x509.Certificate {
	var result []*x509.Certificate
	for _, cert := range certs {
		if !now.After(cert.NotAfter) {
			result = append(result, cert)
		}
	}
	return result
}

func allCertificatesExpired(now time.Time, certs ...*x509.Certificate) bool {
	for _, cert := range certs {
		if !now.After(cert.NotAfter) {
			return false
		}
	}
	return true
}

func validateCert(now time.Time, cert *x509.Certificate, caCerts ...*x509.Certificate) bool {
	if cert == nil || len(cert.Raw) == 0 {
		return false
	}
	added := false
	pool := x509.NewCertPool()
	for _, c := range caCerts {
		if c != nil && len(c.Raw) != 0 {
			pool.AddCert(c)
			added = true
		}
	}
	if !added {
		return false
	}
	_, err := cert.Verify(x509.VerifyOptions{Roots: pool, CurrentTime: now})
	return err == nil
}

func isSecretManagedByKyverno(secret *corev1.Secret) bool {
	if secret != nil {
		labels := secret.GetLabels()
		if labels == nil {
			return false
		}
		if labels[kyverno.LabelCertManagedBy] != kyverno.ValueKyvernoApp {
			return false
		}
	}
	return true
}
