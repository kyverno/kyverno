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
	"time"
)

const certValidityDuration = 10 * 365 * 24 * time.Hour

//TlsCertificateProps Properties of TLS certificate which should be issued for webhook server
type TlsCertificateProps struct {
	Service       string
	Namespace     string
	ApiServerHost string
}

//TlsPemPair The pair of TLS certificate corresponding private key, both in PEM format
type TlsPemPair struct {
	Certificate []byte
	PrivateKey  []byte
}

type KeyPair struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

//TLSGeneratePrivateKey Generates RSA private key
func TLSGeneratePrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

//TLSPrivateKeyToPem Creates PEM block from private key object
func TLSPrivateKeyToPem(rsaKey *rsa.PrivateKey) []byte {
	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}

	return pem.EncodeToMemory(privateKey)
}

func TLSCertificateToPem(certificateDER []byte) []byte {
	certificate := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	}

	return pem.EncodeToMemory(certificate)
}

// GenerateCACert creates the self-signed CA cert and private key
// it will be used to sign the webhook server certificate
func GenerateCACert() (*KeyPair, *TlsPemPair, error) {
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

	pemPair := &TlsPemPair{
		Certificate: TLSCertificateToPem(der),
		PrivateKey:  TLSPrivateKeyToPem(key),
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

// GenerateCertPEM takes the results of GenerateCACert and uses it to create the
// PEM-encoded public certificate and private key, respectively
func GenerateCertPEM(caCert *KeyPair, props TlsCertificateProps, fqdncn bool) (*TlsPemPair, error) {
	now := time.Now()
	begin := now.Add(-1 * time.Hour)
	end := now.Add(certValidityDuration)

	dnsNames := make([]string, 3)
	dnsNames[0] = fmt.Sprintf("%s", props.Service)
	csCommonName := dnsNames[0]

	dnsNames[1] = fmt.Sprintf("%s.%s", props.Service, props.Namespace)
	// The full service name is the CommonName for the certificate
	commonName := GenerateInClusterServiceName(props)
	dnsNames[2] = fmt.Sprintf("%s", commonName)

	if fqdncn {
		// use FQDN as CommonName as a workaournd for https://github.com/kyverno/kyverno/issues/542
		csCommonName = commonName
	}

	var ips []net.IP
	apiServerIP := net.ParseIP(props.ApiServerHost)
	if apiServerIP != nil {
		ips = append(ips, apiServerIP)
	} else {
		dnsNames = append(dnsNames, props.ApiServerHost)
	}

	templ := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: csCommonName,
		},
		DNSNames: dnsNames,
		// IPAddresses:           ips,
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

	pemPair := &TlsPemPair{
		Certificate: TLSCertificateToPem(der),
		PrivateKey:  TLSPrivateKeyToPem(key),
	}

	return pemPair, nil
}

//GenerateInClusterServiceName The generated service name should be the common name for TLS certificate
func GenerateInClusterServiceName(props TlsCertificateProps) string {
	return props.Service + "." + props.Namespace + ".svc"
}

//TlsCertificateGetExpirationDate Gets NotAfter property from raw certificate
func tlsCertificateGetExpirationDate(certData []byte) (*time.Time, error) {
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, errors.New("Failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("Failed to parse certificate: %v" + err.Error())
	}
	return &cert.NotAfter, nil
}

// The certificate is valid for a year, but we update it earlier to avoid using
// an expired certificate in a controller that has been running for a long time
const timeReserveBeforeCertificateExpiration time.Duration = time.Hour * 24 * 30 * 6 // About half a year

//IsTLSPairShouldBeUpdated checks if TLS pair has expited and needs to be updated
func IsTLSPairShouldBeUpdated(tlsPair *TlsPemPair) bool {
	if tlsPair == nil {
		return true
	}

	expirationDate, err := tlsCertificateGetExpirationDate(tlsPair.Certificate)
	if err != nil {
		return true
	}

	// TODO : should use time.Until instead of t.Sub(time.Now()) (gosimple)
	return expirationDate.Sub(time.Now()) < timeReserveBeforeCertificateExpiration
}
