package cosign

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSignature struct {
	annotations map[string]string
	annotErr    error
}

func (m *mockSignature) Annotations() (map[string]string, error) {
	if m.annotErr != nil {
		return nil, m.annotErr
	}
	return m.annotations, nil
}

// Implement other oci.Signature interface methods as no-ops for testing
func (m *mockSignature) Digest() (v1.Hash, error)                            { return v1.Hash{}, nil }
func (m *mockSignature) DiffID() (v1.Hash, error)                            { return v1.Hash{}, nil }
func (m *mockSignature) Compressed() (io.ReadCloser, error)                  { return nil, nil }
func (m *mockSignature) Uncompressed() (io.ReadCloser, error)                { return nil, nil }
func (m *mockSignature) Size() (int64, error)                                { return 0, nil }
func (m *mockSignature) MediaType() (types.MediaType, error)                 { return "", nil }
func (m *mockSignature) Payload() ([]byte, error)                            { return nil, nil }
func (m *mockSignature) Signature() ([]byte, error)                          { return nil, nil }
func (m *mockSignature) Base64Signature() (string, error)                    { return "", nil }
func (m *mockSignature) Cert() (*x509.Certificate, error)                    { return nil, nil }
func (m *mockSignature) Chain() ([]*x509.Certificate, error)                 { return nil, nil }
func (m *mockSignature) Bundle() (*bundle.RekorBundle, error)                { return nil, nil }
func (m *mockSignature) RFC3161Timestamp() (*bundle.RFC3161Timestamp, error) { return nil, nil }

// Test helpers for generating test certificates
func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func generateECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func createCertificate(t *testing.T, template, parent *x509.Certificate, pub, priv interface{}) *x509.Certificate {
	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, pub, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certBytes)
	require.NoError(t, err)
	return cert
}

func certToPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func publicKeyToPEM(key interface{}) []byte {
	pubBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
}

// generateRootCA creates a self-signed root CA certificate
func generateRootCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	key := generateRSAKey(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test Root CA",
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            2,
	}

	cert := createCertificate(t, template, template, &key.PublicKey, key)
	return cert, key
}

// generateIntermediateCA creates an intermediate CA certificate
func generateIntermediateCA(t *testing.T, rootCert *x509.Certificate, rootKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey) {
	key := generateRSAKey(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   "Test Intermediate CA",
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	cert := createCertificate(t, template, rootCert, &key.PublicKey, rootKey)
	return cert, key
}

// generateLeafCert creates a leaf certificate for code signing
func generateLeafCert(t *testing.T, issuerCert *x509.Certificate, issuerKey *rsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	key := generateECDSAKey(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "signer@example.com",
		},
		EmailAddresses:        []string{"signer@example.com"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	cert := createCertificate(t, template, issuerCert, &key.PublicKey, issuerKey)
	return cert, key
}

// generateIntermediateWithoutExtKeyUsage creates an intermediate without ExtKeyUsage
func generateIntermediateWithoutExtKeyUsage(t *testing.T, rootCert *x509.Certificate, rootKey *rsa.PrivateKey) *x509.Certificate {
	key := generateRSAKey(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "Intermediate Without ExtKeyUsage",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	return createCertificate(t, template, rootCert, &key.PublicKey, rootKey)
}

func TestCertPoolFromBytes(t *testing.T) {
	rootCert1, _ := generateRootCA(t)
	rootCert2, _ := generateRootCA(t)

	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		errContains string
		validate    func(t *testing.T, pool *x509.CertPool, cert *x509.Certificate)
	}{
		{
			name:    "valid root certificate",
			input:   certToPEM(rootCert1),
			wantErr: false,
			validate: func(t *testing.T, pool *x509.CertPool, cert *x509.Certificate) {
				// Verify the cert is in the pool by checking if it can verify itself
				opts := x509.VerifyOptions{Roots: pool}
				_, err := cert.Verify(opts)
				assert.NoError(t, err, "Root certificate should be able to verify itself using the pool")
			},
		},
		{
			name:    "multiple certificates",
			input:   append(certToPEM(rootCert1), certToPEM(rootCert2)...),
			wantErr: false,
		},
		{
			name:        "invalid PEM",
			input:       []byte("not a valid PEM"),
			wantErr:     true,
			errContains: "error creating root cert pool",
		},
		{
			name:        "empty data",
			input:       []byte{},
			wantErr:     true,
			errContains: "error creating root cert pool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := certPoolFromBytes(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, pool)
			} else {
				require.NoError(t, err)
				require.NotNil(t, pool)
				if tt.validate != nil {
					tt.validate(t, pool, rootCert1)
				}
			}
		})
	}
}

func TestCertFromBytes(t *testing.T) {
	rootCert1, _ := generateRootCA(t)
	rootCert2, _ := generateRootCA(t)
	pemData := certToPEM(rootCert1)

	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		errContains string
		validate    func(t *testing.T, cert *x509.Certificate)
	}{
		{
			name:    "valid PEM",
			input:   pemData,
			wantErr: false,
			validate: func(t *testing.T, cert *x509.Certificate) {
				assert.Equal(t, rootCert1.Subject.CommonName, cert.Subject.CommonName)
				assert.Equal(t, rootCert1.SerialNumber, cert.SerialNumber)
			},
		},
		{
			name:    "base64 encoded PEM",
			input:   []byte(base64.StdEncoding.EncodeToString(pemData)),
			wantErr: false,
			validate: func(t *testing.T, cert *x509.Certificate) {
				assert.Equal(t, rootCert1.Subject.CommonName, cert.Subject.CommonName)
			},
		},
		{
			name:    "multiple certs returns first",
			input:   append(certToPEM(rootCert1), certToPEM(rootCert2)...),
			wantErr: false,
			validate: func(t *testing.T, cert *x509.Certificate) {
				assert.Equal(t, rootCert1.Subject.CommonName, cert.Subject.CommonName)
			},
		},
		{
			name:    "invalid data",
			input:   []byte("not a certificate"),
			wantErr: true,
		},
		{
			name:    "empty PEM",
			input:   []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := certFromBytes(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cert)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cert)
				if tt.validate != nil {
					tt.validate(t, cert)
				}
			}
		})
	}
}

func TestCertChainFromBytes(t *testing.T) {
	rootCert, rootKey := generateRootCA(t)
	intermediateCert, intermediateKey := generateIntermediateCA(t, rootCert, rootKey)
	leafCert, _ := generateLeafCert(t, intermediateCert, intermediateKey)

	tests := []struct {
		name      string
		input     []byte
		wantErr   bool
		wantCount int
		validate  func(t *testing.T, certs []*x509.Certificate)
	}{
		{
			name: "complete chain",
			input: func() []byte {
				chain := append(certToPEM(leafCert), certToPEM(intermediateCert)...)
				return append(chain, certToPEM(rootCert)...)
			}(),
			wantErr:   false,
			wantCount: 3,
			validate: func(t *testing.T, certs []*x509.Certificate) {
				assert.Equal(t, leafCert.Subject.CommonName, certs[0].Subject.CommonName)
				assert.Equal(t, intermediateCert.Subject.CommonName, certs[1].Subject.CommonName)
				assert.Equal(t, rootCert.Subject.CommonName, certs[2].Subject.CommonName)
			},
		},
		{
			name:      "single cert",
			input:     certToPEM(rootCert),
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:    "invalid data",
			input:   []byte("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs, err := certChainFromBytes(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, certs)
			} else {
				require.NoError(t, err)
				assert.Len(t, certs, tt.wantCount)
				if tt.validate != nil {
					tt.validate(t, certs)
				}
			}
		})
	}
}

func TestSplitCertChain(t *testing.T) {
	rootCert, rootKey := generateRootCA(t)
	intermediateCert, intermediateKey := generateIntermediateCA(t, rootCert, rootKey)
	leafCert, _ := generateLeafCert(t, intermediateCert, intermediateKey)
	leafCert2, _ := generateLeafCert(t, rootCert, rootKey)
	intermediateNoExt := generateIntermediateWithoutExtKeyUsage(t, rootCert, rootKey)

	tests := []struct {
		name              string
		input             []byte
		wantErr           bool
		wantLeaves        int
		wantIntermediates int
		wantRoots         int
		validate          func(t *testing.T, leaves, intermediates, roots []*x509.Certificate)
	}{
		{
			name: "complete chain",
			input: func() []byte {
				chain := append(certToPEM(leafCert), certToPEM(intermediateCert)...)
				return append(chain, certToPEM(rootCert)...)
			}(),
			wantErr:           false,
			wantLeaves:        1,
			wantIntermediates: 1,
			wantRoots:         1,
			validate: func(t *testing.T, leaves, intermediates, roots []*x509.Certificate) {
				// Verify leaf properties
				assert.False(t, leaves[0].IsCA, "Leaf certificate should not be a CA")
				assert.Equal(t, leafCert.Subject.CommonName, leaves[0].Subject.CommonName)

				// Verify intermediate properties
				assert.True(t, intermediates[0].IsCA, "Intermediate should be a CA")
				assert.NotEqual(t, intermediates[0].RawSubject, intermediates[0].RawIssuer, "Intermediate should not be self-signed")
				assert.Equal(t, intermediateCert.Subject.CommonName, intermediates[0].Subject.CommonName)

				// Verify root properties
				assert.True(t, roots[0].IsCA, "Root should be a CA")
				assert.Equal(t, roots[0].RawSubject, roots[0].RawIssuer, "Root should be self-signed")
				assert.Equal(t, rootCert.Subject.CommonName, roots[0].Subject.CommonName)
			},
		},
		{
			name:              "intermediate without ExtKeyUsage",
			input:             append(certToPEM(intermediateNoExt), certToPEM(rootCert)...),
			wantErr:           false,
			wantLeaves:        0,
			wantIntermediates: 1,
			wantRoots:         1,
			validate: func(t *testing.T, leaves, intermediates, roots []*x509.Certificate) {
				// The function should add ExtKeyUsageAny if ExtKeyUsage is empty
				assert.Contains(t, intermediates[0].ExtKeyUsage, x509.ExtKeyUsageTimeStamping,
					"Intermediate without ExtKeyUsage should have ExtKeyUsageAny added")
			},
		},
		{
			name:              "only leaf",
			input:             certToPEM(leafCert),
			wantErr:           false,
			wantLeaves:        1,
			wantIntermediates: 0,
			wantRoots:         0,
			validate: func(t *testing.T, leaves, intermediates, roots []*x509.Certificate) {
				assert.False(t, leaves[0].IsCA)
			},
		},
		{
			name:              "only root",
			input:             certToPEM(rootCert),
			wantErr:           false,
			wantLeaves:        0,
			wantIntermediates: 0,
			wantRoots:         1,
			validate: func(t *testing.T, leaves, intermediates, roots []*x509.Certificate) {
				assert.True(t, roots[0].IsCA)
			},
		},
		{
			name: "multiple leaves",
			input: func() []byte {
				chain := append(certToPEM(leafCert), certToPEM(leafCert2)...)
				return append(chain, certToPEM(rootCert)...)
			}(),
			wantErr:           false,
			wantLeaves:        2,
			wantIntermediates: 0,
			wantRoots:         1,
		},
		{
			name:    "invalid PEM",
			input:   []byte("invalid PEM"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaves, intermediates, roots, err := splitCertChain(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, leaves)
				assert.Nil(t, intermediates)
				assert.Nil(t, roots)
			} else {
				require.NoError(t, err)
				assert.Len(t, leaves, tt.wantLeaves)
				assert.Len(t, intermediates, tt.wantIntermediates)
				assert.Len(t, roots, tt.wantRoots)
				if tt.validate != nil {
					tt.validate(t, leaves, intermediates, roots)
				}
			}
		})
	}
}

func TestDecodePEM(t *testing.T) {
	rsaKey := generateRSAKey(t)
	ecdsaKey := generateECDSAKey(t)
	rootCert, _ := generateRootCA(t)

	tests := []struct {
		name      string
		input     []byte
		hashAlgo  crypto.Hash
		wantErr   bool
		keyType   string
		skipCheck bool
	}{
		{
			name:     "RSA public key",
			input:    publicKeyToPEM(&rsaKey.PublicKey),
			hashAlgo: crypto.SHA256,
			wantErr:  false,
			keyType:  "RSA",
		},
		{
			name:     "ECDSA public key",
			input:    publicKeyToPEM(&ecdsaKey.PublicKey),
			hashAlgo: crypto.SHA256,
			wantErr:  false,
			keyType:  "ECDSA",
		},
		{
			name:     "SHA-256 hash algorithm",
			input:    publicKeyToPEM(&rsaKey.PublicKey),
			hashAlgo: crypto.SHA256,
			wantErr:  false,
		},
		{
			name:     "SHA-384 hash algorithm",
			input:    publicKeyToPEM(&rsaKey.PublicKey),
			hashAlgo: crypto.SHA384,
			wantErr:  false,
		},
		{
			name:     "SHA-512 hash algorithm",
			input:    publicKeyToPEM(&rsaKey.PublicKey),
			hashAlgo: crypto.SHA512,
			wantErr:  false,
		},
		{
			name:     "invalid PEM",
			input:    []byte("not a valid PEM"),
			hashAlgo: crypto.SHA256,
			wantErr:  true,
		},
		{
			name:     "certificate instead of key",
			input:    certToPEM(rootCert),
			hashAlgo: crypto.SHA256,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, err := decodePEM(tt.input, tt.hashAlgo)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, verifier)
			} else {
				require.NoError(t, err)
				require.NotNil(t, verifier)
			}
		})
	}
}

func TestSignatureAlgorithmMap(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		expectedHash crypto.Hash
	}{
		{
			name:         "default is SHA256",
			key:          "",
			expectedHash: crypto.SHA256,
		},
		{
			name:         "sha224",
			key:          "sha224",
			expectedHash: crypto.SHA224,
		},
		{
			name:         "sha256",
			key:          "sha256",
			expectedHash: crypto.SHA256,
		},
		{
			name:         "sha384",
			key:          "sha384",
			expectedHash: crypto.SHA384,
		},
		{
			name:         "sha512",
			key:          "sha512",
			expectedHash: crypto.SHA512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualHash, exists := signatureAlgorithmMap[tt.key]
			assert.True(t, exists, "Algorithm %s should exist in map", tt.key)
			assert.Equal(t, tt.expectedHash, actualHash, "Hash for %s should match", tt.key)
		})
	}
}

func TestCertificateChainValidation(t *testing.T) {
	rootCert, rootKey := generateRootCA(t)
	intermediateCert, intermediateKey := generateIntermediateCA(t, rootCert, rootKey)
	leafCert, _ := generateLeafCert(t, intermediateCert, intermediateKey)

	// Create expired cert
	expiredKey := generateRSAKey(t)
	expiredTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Expired Root CA",
		},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(-24 * time.Hour), // Expired yesterday
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	expiredCert := createCertificate(t, expiredTemplate, expiredTemplate, &expiredKey.PublicKey, expiredKey)

	tests := []struct {
		name         string
		setupPools   func() (roots, intermediates *x509.CertPool, leaf *x509.Certificate)
		wantErr      bool
		errContains  string
		validateOpts func() x509.VerifyOptions
	}{
		{
			name: "valid chain",
			setupPools: func() (roots, intermediates *x509.CertPool, leaf *x509.Certificate) {
				rootPool := x509.NewCertPool()
				rootPool.AddCert(rootCert)
				intermediatePool := x509.NewCertPool()
				intermediatePool.AddCert(intermediateCert)
				return rootPool, intermediatePool, leafCert
			},
			wantErr: false,
			validateOpts: func() x509.VerifyOptions {
				return x509.VerifyOptions{
					KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
				}
			},
		},
		{
			name: "expired certificate",
			setupPools: func() (roots, intermediates *x509.CertPool, leaf *x509.Certificate) {
				rootPool := x509.NewCertPool()
				rootPool.AddCert(expiredCert)
				return rootPool, nil, expiredCert
			},
			wantErr:     true,
			errContains: "expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roots, intermediates, leaf := tt.setupPools()

			opts := x509.VerifyOptions{
				Roots:         roots,
				Intermediates: intermediates,
			}
			if tt.validateOpts != nil {
				customOpts := tt.validateOpts()
				opts.KeyUsages = customOpts.KeyUsages
			}

			chains, err := leaf.Verify(opts)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err, "Valid certificate chain should verify successfully")
				assert.NotEmpty(t, chains, "Should return at least one valid chain")
			}
		})
	}
}

func TestCertificateProperties(t *testing.T) {
	rootCert, rootKey := generateRootCA(t)
	intermediateCert, _ := generateIntermediateCA(t, rootCert, rootKey)
	leafCert, _ := generateLeafCert(t, rootCert, rootKey)

	tests := []struct {
		name     string
		cert     *x509.Certificate
		validate func(t *testing.T, cert *x509.Certificate)
	}{
		{
			name: "code signing usage",
			cert: leafCert,
			validate: func(t *testing.T, cert *x509.Certificate) {
				assert.False(t, cert.IsCA, "Code signing leaf should not be a CA")
				assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageCodeSigning,
					"Code signing certificate should have CodeSigning ExtKeyUsage")
				assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0,
					"Code signing certificate should have DigitalSignature KeyUsage")
			},
		},
		{
			name: "self-signed detection",
			cert: rootCert,
			validate: func(t *testing.T, cert *x509.Certificate) {
				isSelfSigned := string(cert.RawSubject) == string(cert.RawIssuer)
				assert.True(t, isSelfSigned, "Root CA should be self-signed")

				err := cert.CheckSignatureFrom(cert)
				assert.NoError(t, err, "Self-signed certificate should verify its own signature")
			},
		},
		{
			name: "non-self-signed detection",
			cert: intermediateCert,
			validate: func(t *testing.T, cert *x509.Certificate) {
				isSelfSigned := string(cert.RawSubject) == string(cert.RawIssuer)
				assert.False(t, isSelfSigned, "Intermediate CA should not be self-signed")

				// Should be signed by root
				err := cert.CheckSignatureFrom(rootCert)
				assert.NoError(t, err, "Intermediate should be signed by root")

				// Should NOT verify against itself
				err = cert.CheckSignatureFrom(cert)
				assert.Error(t, err, "Non-self-signed certificate should not verify against itself")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.cert)
		})
	}
}

func TestCheckSignatureAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		sig         *mockSignature
		expected    map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "matching annotations",
			sig: &mockSignature{
				annotations: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name: "all annotations match",
			sig: &mockSignature{
				annotations: map[string]string{
					"io.kyverno.image": "myimage:v1",
					"builder":          "github-actions",
				},
			},
			expected: map[string]string{
				"io.kyverno.image": "myimage:v1",
				"builder":          "github-actions",
			},
			wantErr: false,
		},
		{
			name: "mismatched value",
			sig: &mockSignature{
				annotations: map[string]string{
					"key1": "value1",
					"key2": "wrongvalue",
				},
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "expectedvalue",
			},
			wantErr:     true,
			errContains: "annotations mismatch",
		},
		{
			name: "missing key",
			sig: &mockSignature{
				annotations: map[string]string{
					"key1": "value1",
				},
			},
			expected: map[string]string{
				"key1":       "value1",
				"missingkey": "somevalue",
			},
			wantErr:     true,
			errContains: "annotations mismatch",
		},
		{
			name: "empty expected",
			sig: &mockSignature{
				annotations: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: map[string]string{},
			wantErr:  false,
		},
		{
			name: "empty signature",
			sig: &mockSignature{
				annotations: map[string]string{},
			},
			expected: map[string]string{
				"key1": "value1",
			},
			wantErr:     true,
			errContains: "annotations mismatch",
		},
		{
			name: "annotations fetch error",
			sig: &mockSignature{
				annotErr: errors.New("failed to fetch annotations"),
			},
			expected: map[string]string{
				"key1": "value1",
			},
			wantErr:     true,
			errContains: "failed to fetch annotation from signature",
		},
		{
			name: "cosign standard annotations",
			sig: &mockSignature{
				annotations: map[string]string{
					"dev.cosignproject.cosign/signature": "MEUCIQDxUX...",
					"dev.sigstore.cosign/bundle":         `{"SignedEntryTimestamp":"..."}`,
				},
			},
			expected: map[string]string{
				"dev.cosignproject.cosign/signature": "MEUCIQDxUX...",
			},
			wantErr: false,
		},
		{
			name: "case sensitivity",
			sig: &mockSignature{
				annotations: map[string]string{
					"Key1": "Value1",
				},
			},
			expected: map[string]string{
				"key1": "Value1", // Different case in key
			},
			wantErr:     true,
			errContains: "annotations mismatch",
		},
		{
			name: "extra annotations in signature",
			sig: &mockSignature{
				annotations: map[string]string{
					"key1":  "value1",
					"key2":  "value2",
					"extra": "annotation",
				},
			},
			expected: map[string]string{
				"key1": "value1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSignatureAnnotations(tt.sig, tt.expected)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
