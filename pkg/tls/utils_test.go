package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyGenerator is a function that generates a key and returns the PEM-encoded bytes and expected type
type keyGenerator func() ([]byte, string, error)

func generateRSAPKCS1() ([]byte, string, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(rsaKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs1Bytes,
	})
	return pemBytes, "*rsa.PrivateKey", nil
}

func generateRSAPKCS8() ([]byte, string, error) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})
	return pemBytes, "*rsa.PrivateKey", nil
}

func generateECDSASEC1P256() ([]byte, string, error) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	sec1Bytes, err := x509.MarshalECPrivateKey(ecdsaKey)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: sec1Bytes,
	})
	return pemBytes, "*ecdsa.PrivateKey", nil
}

func generateECDSAPKCS8P256() ([]byte, string, error) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})
	return pemBytes, "*ecdsa.PrivateKey", nil
}

func generateECDSAPKCS8P384() ([]byte, string, error) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})
	return pemBytes, "*ecdsa.PrivateKey", nil
}

func generateECDSAPKCS8P521() ([]byte, string, error) {
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, "", err
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})
	return pemBytes, "*ecdsa.PrivateKey", nil
}

func generateEd25519PKCS8() ([]byte, string, error) {
	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(ed25519Key)
	if err != nil {
		return nil, "", err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})
	return pemBytes, "ed25519.PrivateKey", nil
}

func TestPemToPrivateKey(t *testing.T) {
	tests := []struct {
		name         string
		keyGenerator keyGenerator
		wantErr      bool
	}{
		{
			name:         "RSA PKCS#1",
			keyGenerator: generateRSAPKCS1,
		},
		{
			name:         "RSA PKCS#8",
			keyGenerator: generateRSAPKCS8,
		},
		{
			name:         "ECDSA SEC1 P-256",
			keyGenerator: generateECDSASEC1P256,
		},
		{
			name:         "ECDSA PKCS#8 P-256",
			keyGenerator: generateECDSAPKCS8P256,
		},
		{
			name:         "ECDSA PKCS#8 P-384",
			keyGenerator: generateECDSAPKCS8P384,
		},
		{
			name:         "ECDSA PKCS#8 P-521",
			keyGenerator: generateECDSAPKCS8P521,
		},
		{
			name:         "Ed25519 PKCS#8",
			keyGenerator: generateEd25519PKCS8,
		},
		{
			name: "Invalid PEM data",
			keyGenerator: func() ([]byte, string, error) {
				return []byte("not a valid PEM"), "invalid", nil
			},
			wantErr: true,
		},
		{
			name: "Valid PEM but invalid key data",
			keyGenerator: func() ([]byte, string, error) {
				invalid := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: []byte("invalid key data"),
				})
				return invalid, "invalid", nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pemBytes, expectedType, err := tt.keyGenerator()
			require.NoError(t, err, "failed to generate key")

			parsedKey, err := pemToPrivateKey(pemBytes)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err, "failed to parse key")
			assert.Contains(t, getKeyTypeName(parsedKey), expectedType, "unexpected key type")
		})
	}
}

func TestPrivateKeyToPem(t *testing.T) {
	tests := []struct {
		name         string
		generateKey  func() (crypto.PrivateKey, error)
		expectedType string
	}{
		{
			name: "RSA",
			generateKey: func() (crypto.PrivateKey, error) {
				return rsa.GenerateKey(rand.Reader, 2048)
			},
			expectedType: "*rsa.PrivateKey",
		},
		{
			name: "ECDSA P-256",
			generateKey: func() (crypto.PrivateKey, error) {
				return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			},
			expectedType: "*ecdsa.PrivateKey",
		},
		{
			name: "Ed25519",
			generateKey: func() (crypto.PrivateKey, error) {
				_, key, err := ed25519.GenerateKey(rand.Reader)
				return key, err
			},
			expectedType: "ed25519.PrivateKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := tt.generateKey()
			require.NoError(t, err, "failed to generate key")

			pemBytes, err := privateKeyToPem(key)
			require.NoError(t, err, "failed to convert key to PEM")

			// Parse it back to verify round-trip
			parsedKey, err := pemToPrivateKey(pemBytes)
			require.NoError(t, err, "failed to parse key back")
			assert.Contains(t, getKeyTypeName(parsedKey), tt.expectedType, "unexpected key type after round-trip")
		})
	}
}

func TestPemToPrivateKey_ECDSACurves(t *testing.T) {
	tests := []struct {
		name          string
		curve         elliptic.Curve
		expectedCurve elliptic.Curve
	}{
		{
			name:          "P-384",
			curve:         elliptic.P384(),
			expectedCurve: elliptic.P384(),
		},
		{
			name:          "P-521",
			curve:         elliptic.P521(),
			expectedCurve: elliptic.P521(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecdsaKey, err := ecdsa.GenerateKey(tt.curve, rand.Reader)
			require.NoError(t, err, "failed to generate ECDSA key")

			pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(ecdsaKey)
			require.NoError(t, err, "failed to marshal ECDSA key to PKCS#8")

			pemBytes := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: pkcs8Bytes,
			})

			parsedKey, err := pemToPrivateKey(pemBytes)
			require.NoError(t, err, "failed to parse ECDSA key")

			ecKey, ok := parsedKey.(*ecdsa.PrivateKey)
			require.True(t, ok, "expected *ecdsa.PrivateKey, got %T", parsedKey)
			assert.Equal(t, tt.expectedCurve, ecKey.Curve, "unexpected curve")
		})
	}
}

// getKeyTypeName returns a string representation of the key type for assertion messages
func getKeyTypeName(key crypto.PrivateKey) string {
	switch key.(type) {
	case *rsa.PrivateKey:
		return "*rsa.PrivateKey"
	case *ecdsa.PrivateKey:
		return "*ecdsa.PrivateKey"
	case ed25519.PrivateKey:
		return "ed25519.PrivateKey"
	default:
		return "unknown"
	}
}
