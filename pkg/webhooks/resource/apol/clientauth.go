package apol

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
)

// ClientAuth authenticates callers of the authorization endpoints by their
// TLS client certificate. The server requests a certificate without verifying
// it during the handshake, so admission webhook callers are not affected; the
// authorization handlers verify it here.
type ClientAuth struct {
	roots *x509.CertPool
	// names, when set, restricts callers to certificates whose subject common
	// name or a DNS SAN is in the list.
	names []string
}

// LoadClientAuth reads PEM CA certificates from caFile.
func LoadClientAuth(caFile string, names []string) (*ClientAuth, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading client CA file: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("client CA file %s has no PEM certificates", caFile)
	}
	return &ClientAuth{roots: roots, names: names}, nil
}

// Authenticate returns an error unless the request carries a client
// certificate that chains to the configured roots and has an allowed name.
func (a *ClientAuth) Authenticate(r *http.Request) error {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return errors.New("client certificate is required")
	}
	leaf := r.TLS.PeerCertificates[0]
	intermediates := x509.NewCertPool()
	for _, cert := range r.TLS.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         a.roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return fmt.Errorf("verifying client certificate: %w", err)
	}
	if len(a.names) == 0 || slices.Contains(a.names, leaf.Subject.CommonName) {
		return nil
	}
	for _, name := range leaf.DNSNames {
		if slices.Contains(a.names, name) {
			return nil
		}
	}
	return fmt.Errorf("client certificate name %q is not allowed", leaf.Subject.CommonName)
}
