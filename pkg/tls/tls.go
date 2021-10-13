package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// CertRenewalInterval is the renewal interval for rootCA
const CertRenewalInterval time.Duration = 12 * time.Hour

// CertValidityDuration is the valid duration for a new cert
const CertValidityDuration time.Duration = 365 * 24 * time.Hour

// CertificateProps Properties of TLS certificate which should be issued for webhook server
type CertificateProps struct {
	Service       string
	Namespace     string
	APIServerHost string
	ServerIP      string
}

// PemPair The pair of TLS certificate corresponding private key, both in PEM format
type PemPair struct {
	Certificate []byte
	PrivateKey  []byte
}

// KeyPair ...
type KeyPair struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

// GeneratePrivateKey Generates RSA private key
func GeneratePrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
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

// GenerateCACert creates the self-signed CA cert and private key
// it will be used to sign the webhook server certificate
func GenerateCACert(certValidityDuration time.Duration) (*KeyPair, *PemPair, error) {
	now := time.Now()
	begin := now.Add(-1 * time.Hour)
	end := now.Add(certValidityDuration)

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

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating key: %v", err)
	}

	der, err := x509.CreateCertificate(rand.Reader, templ, templ, key.Public(), key)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating certificate: %v", err)
	}

	pemPair := &PemPair{
		Certificate: CertificateToPem(der),
		PrivateKey:  PrivateKeyToPem(key),
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing certificate %v", err)
	}

	caCert := &KeyPair{
		Cert: cert,
		Key:  key,
	}

	return caCert, pemPair, nil
}

// GenerateCertPem takes the results of GenerateCACert and uses it to create the
// PEM-encoded public certificate and private key, respectively
func GenerateCertPem(caCert *KeyPair, props CertificateProps, serverIP string, certValidityDuration time.Duration) (*PemPair, error) {
	now := time.Now()
	begin := now.Add(-1 * time.Hour)
	end := now.Add(certValidityDuration)

	dnsNames := make([]string, 3)
	dnsNames[0] = props.Service
	csCommonName := dnsNames[0]

	dnsNames[1] = fmt.Sprintf("%s.%s", props.Service, props.Namespace)
	// The full service name is the CommonName for the certificate
	commonName := generateInClusterServiceName(props)
	dnsNames[2] = commonName

	var ips []net.IP
	apiServerIP := net.ParseIP(props.APIServerHost)
	if apiServerIP != nil {
		ips = append(ips, apiServerIP)
	} else {
		dnsNames = append(dnsNames, props.APIServerHost)
	}

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
			CommonName: csCommonName,
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

	pemPair := &PemPair{
		Certificate: CertificateToPem(der),
		PrivateKey:  PrivateKeyToPem(key),
	}

	return pemPair, nil
}

//GenerateInClusterServiceName The generated service name should be the common name for TLS certificate
func generateInClusterServiceName(props CertificateProps) string {
	return props.Service + "." + props.Namespace + ".svc"
}

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

// The certificate is valid for a year, but we update it earlier to avoid using
// an expired certificate in a controller that has been running for a long time
const timeReserveBeforeCertificateExpiration time.Duration = time.Hour * 24 * 30 * 6 // About half a year

//IsTLSPairShouldBeUpdated checks if TLS pair has expited and needs to be updated
func IsTLSPairShouldBeUpdated(tlsPair *PemPair) bool {
	if tlsPair == nil {
		return true
	}

	expirationDate, err := tlsCertificateGetExpirationDate(tlsPair.Certificate)
	if err != nil {
		return true
	}
	return time.Until(*expirationDate) < timeReserveBeforeCertificateExpiration
}
