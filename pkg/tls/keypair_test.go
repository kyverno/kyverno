package tls

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyAlgorithms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected KeyAlgorithm
		exists   bool
	}{
		{
			name:     "RSA uppercase",
			input:    "RSA",
			expected: RSA,
			exists:   true,
		},
		{
			name:     "ECDSA uppercase",
			input:    "ECDSA",
			expected: ECDSA,
			exists:   true,
		},
		{
			name:     "Ed25519 uppercase",
			input:    "ED25519",
			expected: Ed25519,
			exists:   true,
		},
		{
			name:     "Empty string defaults to RSA",
			input:    "",
			expected: RSA,
			exists:   true,
		},
		{
			name:   "Invalid algorithm",
			input:  "INVALID",
			exists: false,
		},
		{
			name:   "Unknown algorithm",
			input:  "DSA",
			exists: false,
		},
		{
			name:   "Lowercase not in map",
			input:  "rsa",
			exists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := KeyAlgorithms[tt.input]
			assert.Equal(t, tt.exists, ok)
			if tt.exists {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
		keyType   string
	}{
		{
			name:      "RSA",
			algorithm: RSA,
			keyType:   "*rsa.PrivateKey",
		},
		{
			name:      "ECDSA",
			algorithm: ECDSA,
			keyType:   "*ecdsa.PrivateKey",
		},
		{
			name:      "Ed25519",
			algorithm: Ed25519,
			keyType:   "ed25519.PrivateKey",
		},
		{
			name:      "Empty defaults to RSA",
			algorithm: "",
			keyType:   "*rsa.PrivateKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := generatePrivateKey(tt.algorithm)
			require.NoError(t, err)
			assert.Equal(t, tt.keyType, getKeyTypeName(key))
		})
	}
}

func TestGenerateCA(t *testing.T) {
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
		keyType   string
	}{
		{
			name:      "RSA CA",
			algorithm: RSA,
			keyType:   "*rsa.PrivateKey",
		},
		{
			name:      "ECDSA CA",
			algorithm: ECDSA,
			keyType:   "*ecdsa.PrivateKey",
		},
		{
			name:      "Ed25519 CA",
			algorithm: Ed25519,
			keyType:   "ed25519.PrivateKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validity := 24 * time.Hour

			key, cert, err := generateCA(nil, validity, tt.algorithm)
			require.NoError(t, err, "failed to generate CA")

			// Check key type
			assert.Equal(t, tt.keyType, getKeyTypeName(key))

			// Check certificate properties
			assert.True(t, cert.IsCA, "certificate should be a CA")
			assert.Equal(t, "*.kyverno.svc", cert.Subject.CommonName)
			assert.True(t, cert.BasicConstraintsValid)

			// Check key usage
			assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0, "should have DigitalSignature key usage")
			assert.True(t, cert.KeyUsage&x509.KeyUsageCertSign != 0, "should have CertSign key usage")

			// RSA keys should also have KeyEncipherment
			if tt.algorithm == RSA {
				assert.True(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0, "RSA should have KeyEncipherment key usage")
			}

			// Check validity period
			assert.True(t, cert.NotAfter.After(cert.NotBefore))
		})
	}
}

func TestGenerateCA_WithExistingKey(t *testing.T) {
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
	}{
		{
			name:      "RSA with existing key",
			algorithm: RSA,
		},
		{
			name:      "ECDSA with existing key",
			algorithm: ECDSA,
		},
		{
			name:      "Ed25519 with existing key",
			algorithm: Ed25519,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validity := 24 * time.Hour

			// Generate first CA
			key1, _, err := generateCA(nil, validity, tt.algorithm)
			require.NoError(t, err)

			// Generate second CA with same key
			key2, cert2, err := generateCA(key1, validity, tt.algorithm)
			require.NoError(t, err)

			// Keys should be the same
			assert.Equal(t, key1, key2)
			assert.True(t, cert2.IsCA)
		})
	}
}

func TestGenerateCA_KeyTypeMismatch(t *testing.T) {
	validity := 24 * time.Hour

	// Generate RSA key
	rsaKey, err := generatePrivateKey(RSA)
	require.NoError(t, err)

	// Try to use it with ECDSA algorithm - should fail
	_, _, err = generateCA(rsaKey, validity, ECDSA)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match requested algorithm")
}

func TestGenerateTLS(t *testing.T) {
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
		keyType   string
	}{
		{
			name:      "RSA TLS",
			algorithm: RSA,
			keyType:   "*rsa.PrivateKey",
		},
		{
			name:      "ECDSA TLS",
			algorithm: ECDSA,
			keyType:   "*ecdsa.PrivateKey",
		},
		{
			name:      "Ed25519 TLS",
			algorithm: Ed25519,
			keyType:   "ed25519.PrivateKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caValidity := 365 * 24 * time.Hour
			tlsValidity := 24 * time.Hour

			// Generate CA first
			caKey, caCert, err := generateCA(nil, caValidity, tt.algorithm)
			require.NoError(t, err)

			// Generate TLS certificate
			commonName := "kyverno-admission-controller.kyverno.svc"
			dnsNames := []string{
				"kyverno-admission-controller",
				"kyverno-admission-controller.kyverno",
				"kyverno-admission-controller.kyverno.svc",
			}

			tlsKey, tlsCert, err := generateTLS("", caCert, caKey, tlsValidity, commonName, dnsNames, tt.algorithm)
			require.NoError(t, err)

			// Check key type
			assert.Equal(t, tt.keyType, getKeyTypeName(tlsKey))

			// Check certificate properties
			assert.False(t, tlsCert.IsCA, "TLS certificate should not be a CA")
			assert.Equal(t, commonName, tlsCert.Subject.CommonName)
			assert.Equal(t, dnsNames, tlsCert.DNSNames)

			// Check key usage
			assert.True(t, tlsCert.KeyUsage&x509.KeyUsageDigitalSignature != 0)
			if tt.algorithm == RSA {
				assert.True(t, tlsCert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
			}

			// Check extended key usage
			assert.Contains(t, tlsCert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

			// Verify the TLS certificate is signed by the CA
			roots := x509.NewCertPool()
			roots.AddCert(caCert)
			_, err = tlsCert.Verify(x509.VerifyOptions{Roots: roots})
			require.NoError(t, err, "TLS certificate should be verifiable with CA")
		})
	}
}

func TestGenerateTLS_WithServerIP(t *testing.T) {
	caValidity := 365 * 24 * time.Hour
	tlsValidity := 24 * time.Hour

	caKey, caCert, err := generateCA(nil, caValidity, RSA)
	require.NoError(t, err)

	tests := []struct {
		name           string
		serverIP       string
		expectedIPLen  int
		expectedDNSLen int
	}{
		{
			name:           "With IP address",
			serverIP:       "192.168.1.100",
			expectedIPLen:  1,
			expectedDNSLen: 0, // IP addresses don't add to DNS names
		},
		{
			name:           "With hostname",
			serverIP:       "my-server.example.com",
			expectedIPLen:  0,
			expectedDNSLen: 1, // serverIP is added as DNS name
		},
		{
			name:           "With IP and port",
			serverIP:       "192.168.1.100:8443",
			expectedIPLen:  1,
			expectedDNSLen: 0, // IP addresses don't add to DNS names
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsKey, tlsCert, err := generateTLS(tt.serverIP, caCert, caKey, tlsValidity, "test-server", nil, RSA)
			require.NoError(t, err)
			require.NotNil(t, tlsKey)

			assert.Len(t, tlsCert.IPAddresses, tt.expectedIPLen, "unexpected number of IP addresses")
			assert.Len(t, tlsCert.DNSNames, tt.expectedDNSLen, "unexpected number of DNS names")
		})
	}
}

func TestGetKeyAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
		expected  KeyAlgorithm
	}{
		{
			name:      "RSA",
			algorithm: RSA,
			expected:  RSA,
		},
		{
			name:      "ECDSA",
			algorithm: ECDSA,
			expected:  ECDSA,
		},
		{
			name:      "Ed25519",
			algorithm: Ed25519,
			expected:  Ed25519,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := generatePrivateKey(tt.algorithm)
			require.NoError(t, err)

			result := getKeyAlgorithm(key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPublicKey(t *testing.T) {
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
	}{
		{
			name:      "RSA",
			algorithm: RSA,
		},
		{
			name:      "ECDSA",
			algorithm: ECDSA,
		},
		{
			name:      "Ed25519",
			algorithm: Ed25519,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey, err := generatePrivateKey(tt.algorithm)
			require.NoError(t, err)

			publicKey, err := getPublicKey(privateKey)
			require.NoError(t, err)
			require.NotNil(t, publicKey)

			// Verify the public key type matches the private key
			switch tt.algorithm {
			case RSA:
				_, ok := publicKey.(*rsa.PublicKey)
				assert.True(t, ok, "expected *rsa.PublicKey")
			case ECDSA:
				_, ok := publicKey.(*ecdsa.PublicKey)
				assert.True(t, ok, "expected *ecdsa.PublicKey")
			case Ed25519:
				_, ok := publicKey.(ed25519.PublicKey)
				assert.True(t, ok, "expected ed25519.PublicKey")
			}
		})
	}
}

func TestRoundTrip_KeyGeneration(t *testing.T) {
	// Test that we can generate a CA and TLS cert, encode them to PEM,
	// and decode them back successfully
	tests := []struct {
		name      string
		algorithm KeyAlgorithm
	}{
		{
			name:      "RSA round trip",
			algorithm: RSA,
		},
		{
			name:      "ECDSA round trip",
			algorithm: ECDSA,
		},
		{
			name:      "Ed25519 round trip",
			algorithm: Ed25519,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate CA
			caKey, caCert, err := generateCA(nil, 365*24*time.Hour, tt.algorithm)
			require.NoError(t, err)

			// Encode CA key to PEM
			caPEM, err := privateKeyToPem(caKey)
			require.NoError(t, err)

			// Decode CA key from PEM
			decodedCAKey, err := pemToPrivateKey(caPEM)
			require.NoError(t, err)
			assert.Equal(t, getKeyAlgorithm(caKey), getKeyAlgorithm(decodedCAKey))

			// Generate TLS cert
			tlsKey, tlsCert, err := generateTLS("", caCert, caKey, 24*time.Hour, "test.kyverno.svc", nil, tt.algorithm)
			require.NoError(t, err)

			// Encode TLS key to PEM
			tlsPEM, err := privateKeyToPem(tlsKey)
			require.NoError(t, err)

			// Decode TLS key from PEM
			decodedTLSKey, err := pemToPrivateKey(tlsPEM)
			require.NoError(t, err)
			assert.Equal(t, getKeyAlgorithm(tlsKey), getKeyAlgorithm(decodedTLSKey))

			// Verify the TLS cert is valid
			roots := x509.NewCertPool()
			roots.AddCert(caCert)
			_, err = tlsCert.Verify(x509.VerifyOptions{Roots: roots})
			require.NoError(t, err)
		})
	}
}
