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

// KeyPair ...
type KeyPair struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

// GenerateCA creates the self-signed CA cert and private key
// it will be used to sign the webhook server certificate
func GenerateCA(certValidityDuration time.Duration) (*KeyPair, error) {
	now := time.Now()
	begin, end := now.Add(-1*time.Hour), now.Add(certValidityDuration)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("error generating key: %v", err)
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
		return nil, fmt.Errorf("error creating certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate %v", err)
	}
	return &KeyPair{
		Cert: cert,
		Key:  key,
	}, nil
}

// GenerateCert takes the results of GenerateCACert and uses it to create the
// PEM-encoded public certificate and private key, respectively
func GenerateCert(caCert *KeyPair, props *CertificateProps, serverIP string, certValidityDuration time.Duration) (*KeyPair, error) {
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
		return nil, fmt.Errorf("error generating key for webhook %v", err)
	}
	der, err := x509.CreateCertificate(rand.Reader, templ, caCert.Cert, key.Public(), caCert.Key)
	if err != nil {
		return nil, fmt.Errorf("error creating certificate for webhook %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("error parsing webhook certificate %v", err)
	}
	return &KeyPair{
		Cert: cert,
		Key:  key,
	}, nil
}
