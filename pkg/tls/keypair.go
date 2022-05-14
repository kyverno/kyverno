package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/config"
)

// generateCA creates the self-signed CA cert and private key
// it will be used to sign the webhook server certificate
func generateCA(key *rsa.PrivateKey, certValidityDuration time.Duration) (*rsa.PrivateKey, *x509.Certificate, error) {
	now := time.Now()
	begin, end := now.Add(-1*time.Hour), now.Add(certValidityDuration)
	if key == nil {
		newKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, nil, err
		}
		key = newKey
	}
	templ := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName: "*.kyverno.svc",
		},
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, templ, templ, key.Public(), key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}

// generateTLS takes the results of GenerateCACert and uses it to create the
// PEM-encoded public certificate and private key, respectively
func generateTLS(props *certificateProps, serverIP string, caCert *x509.Certificate, caKey *rsa.PrivateKey, certValidityDuration time.Duration) (*rsa.PrivateKey, *x509.Certificate, error) {
	now := time.Now()
	begin, end := now.Add(-1*time.Hour), now.Add(certValidityDuration)
	dnsNames := []string{
		config.KyvernoServiceName(),
		fmt.Sprintf("%s.%s", config.KyvernoServiceName(), config.KyvernoNamespace()),
		InClusterServiceName(),
	}
	var ips []net.IP
	if serverIP != "" {
		if strings.Contains(serverIP, ":") {
			host, _, _ := net.SplitHostPort(serverIP)
			serverIP = host
		}
		ip := net.ParseIP(serverIP)
		ips = append(ips, ip)
	}
	templ := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: config.KyvernoServiceName(),
		},
		DNSNames:              dnsNames,
		IPAddresses:           ips,
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, templ, caCert, key.Public(), caKey)
	if err != nil {
		logger.Error(err, "create certificate failed")
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		logger.Error(err, "parse certificate failed")
		return nil, nil, err
	}
	return key, cert, nil
}
