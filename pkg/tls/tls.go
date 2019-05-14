package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"net"
	"time"

	certificates "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Properties of TLS certificate which should be issued for webhook server
type TlsCertificateProps struct {
	Service       string
	Namespace     string
	ApiServerHost string
}

// The pair of TLS certificate corresponding private key, both in PEM format
type TlsPemPair struct {
	Certificate []byte
	PrivateKey  []byte
}

// Generates RSA private key
func TlsGeneratePrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// Creates PEM block from private key object
func TlsPrivateKeyToPem(rsaKey *rsa.PrivateKey) []byte {
	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}

	return pem.EncodeToMemory(privateKey)
}

// Creates PEM block from raw certificate request
func TlsCertificateRequestToPem(csrRaw []byte) []byte {
	csrBlock := &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrRaw,
	}

	return pem.EncodeToMemory(csrBlock)
}

// Generates raw certificate signing request
func TlsCertificateGenerateRequest(privateKey *rsa.PrivateKey, props TlsCertificateProps) (*certificates.CertificateSigningRequest, error) {
	dnsNames := make([]string, 3)
	dnsNames[0] = props.Service
	dnsNames[1] = props.Service + "." + props.Namespace
	// The full service name is the CommonName for the certificate
	commonName := GenerateInClusterServiceName(props)
	dnsNames[2] = commonName

	var ips []net.IP
	apiServerIp := net.ParseIP(props.ApiServerHost)
	if apiServerIp != nil {
		ips = append(ips, apiServerIp)
	} else {
		dnsNames = append(dnsNames, props.ApiServerHost)
	}

	csrTemplate := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: props.Service, //commonName,
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
		DNSNames:           dnsNames,
		IPAddresses:        ips,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, privateKey)
	if err != nil {
		return nil, err
	}

	return &certificates.CertificateSigningRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "certificates.k8s.io/v1beta1",
			Kind:       "CertificateSigningRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: props.Service + "." + props.Namespace + ".cert-request",
		},
		Spec: certificates.CertificateSigningRequestSpec{
			Request: TlsCertificateRequestToPem(csrBytes),
			Groups:  []string{"system:masters", "system:authenticated"},
			Usages: []certificates.KeyUsage{
				certificates.UsageDigitalSignature,
				certificates.UsageKeyEncipherment,
				certificates.UsageServerAuth,
				certificates.UsageClientAuth,
			},
		},
	}, nil
}

// The generated service name should be the common name for TLS certificate
func GenerateInClusterServiceName(props TlsCertificateProps) string {
	return props.Service + "." + props.Namespace + ".svc"
}

//Gets NotAfter property from raw certificate
func TlsCertificateGetExpirationDate(certData []byte) (*time.Time, error) {
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

func IsTlsPairShouldBeUpdated(tlsPair *TlsPemPair) bool {
	if tlsPair == nil {
		return true
	}

	expirationDate, err := TlsCertificateGetExpirationDate(tlsPair.Certificate)
	if err != nil {
		return true
	}

	return expirationDate.Sub(time.Now()) < timeReserveBeforeCertificateExpiration
}
