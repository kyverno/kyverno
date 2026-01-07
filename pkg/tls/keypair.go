package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// KeyAlgorithm represents the type of key algorithm to use for certificate generation
type KeyAlgorithm string

const (
	// RSA uses RSA 2048-bit keys (default, for backward compatibility)
	RSA KeyAlgorithm = "RSA"
	// ECDSA uses ECDSA P-256 keys
	ECDSA KeyAlgorithm = "ECDSA"
	// Ed25519 uses Ed25519 keys
	Ed25519 KeyAlgorithm = "Ed25519"
)

// KeyAlgorithms maps string representations to KeyAlgorithm values
var KeyAlgorithms = map[string]KeyAlgorithm{
	"RSA":     RSA,
	"":        RSA, // default
	"ECDSA":   ECDSA,
	"ED25519": Ed25519,
}

// generatePrivateKey generates a new private key based on the specified algorithm
func generatePrivateKey(algorithm KeyAlgorithm) (crypto.PrivateKey, error) {
	switch algorithm {
	case ECDSA:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case Ed25519:
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		return privateKey, err
	case RSA, "":
		return rsa.GenerateKey(rand.Reader, 2048)
	default:
		return nil, fmt.Errorf("unsupported key algorithm: %s", algorithm)
	}
}

// getPublicKey extracts the public key from a private key
func getPublicKey(key crypto.PrivateKey) (crypto.PublicKey, error) {
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("private key does not implement crypto.Signer")
	}
	return signer.Public(), nil
}

// getKeyAlgorithm returns the algorithm of an existing key
func getKeyAlgorithm(key crypto.PrivateKey) KeyAlgorithm {
	switch key.(type) {
	case *rsa.PrivateKey:
		return RSA
	case *ecdsa.PrivateKey:
		return ECDSA
	case ed25519.PrivateKey:
		return Ed25519
	default:
		return ""
	}
}

// generateCA creates the self-signed CA cert and private key
// it will be used to sign the webhook server certificate
func generateCA(key crypto.PrivateKey, certValidityDuration time.Duration, algorithm KeyAlgorithm) (crypto.PrivateKey, *x509.Certificate, error) {
	now := time.Now()
	begin, end := now.Add(-1*time.Hour), now.Add(certValidityDuration)

	if key == nil {
		newKey, err := generatePrivateKey(algorithm)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
		}
		key = newKey
	} else {
		// Verify existing key matches the requested algorithm
		existingAlgorithm := getKeyAlgorithm(key)
		if existingAlgorithm != algorithm {
			return nil, nil, fmt.Errorf("existing key algorithm (%s) does not match requested algorithm (%s), cannot regenerate CA with different key type", existingAlgorithm, algorithm)
		}
	}

	publicKey, err := getPublicKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get public key from private key: %w", err)
	}

	// Set appropriate key usage based on algorithm
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
	// RSA keys also support key encipherment
	if _, isRSA := key.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	templ := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName: "*.kyverno.svc",
		},
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              keyUsage,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, templ, templ, publicKey, key)
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
func generateTLS(server string, caCert *x509.Certificate, caKey crypto.PrivateKey, certValidityDuration time.Duration, commonName string, dnsNames []string, algorithm KeyAlgorithm) (crypto.PrivateKey, *x509.Certificate, error) {
	now := time.Now()
	begin, end := now.Add(-1*time.Hour), now.Add(certValidityDuration)
	var ips []net.IP
	if server != "" {
		serverHost := server
		if strings.Contains(serverHost, ":") {
			host, _, err := net.SplitHostPort(serverHost)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to split server host/port (%w)", err)
			}
			serverHost = host
		}
		ip := net.ParseIP(serverHost)
		if ip == nil || ip.IsUnspecified() {
			dnsNames = append(dnsNames, serverHost)
		} else {
			ips = append(ips, ip)
		}
	}

	privateKey, err := generatePrivateKey(algorithm)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey, err := getPublicKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get public key from private key: %w", err)
	}

	// Set appropriate key usage based on algorithm
	keyUsage := x509.KeyUsageDigitalSignature
	// RSA keys also support key encipherment
	if _, isRSA := privateKey.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	templ := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames:              dnsNames,
		IPAddresses:           ips,
		NotBefore:             begin,
		NotAfter:              end,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, templ, caCert, publicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, cert, nil
}
