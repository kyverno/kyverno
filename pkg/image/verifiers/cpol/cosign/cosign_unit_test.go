package cosign

import (
	"crypto"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/kyverno/kyverno/pkg/images"
	cosignPkg "github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/cosign/attestation"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

func TestExtractDigest(t *testing.T) {
	tests := []struct {
		name       string
		imgRef     string
		payload    []payload.SimpleContainerImage
		wantDigest string
		wantErr    bool
	}{
		{
			name:   "valid digest",
			imgRef: "ghcr.io/test/image:v1",
			payload: []payload.SimpleContainerImage{
				{
					Critical: payload.Critical{
						Image: payload.Image{
							DockerManifestDigest: "sha256:abc123",
						},
					},
				},
			},
			wantDigest: "sha256:abc123",
		},
		{
			name:       "empty payload",
			imgRef:     "ghcr.io/test/image:v1",
			payload:    []payload.SimpleContainerImage{},
			wantDigest: "",
			wantErr:    true,
		},
		{
			name:   "empty digest in payload",
			imgRef: "ghcr.io/test/image:v1",
			payload: []payload.SimpleContainerImage{
				{
					Critical: payload.Critical{
						Image: payload.Image{
							DockerManifestDigest: "",
						},
					},
				},
			},
			wantDigest: "",
			wantErr:    true,
		},
		{
			name:   "multiple payloads returns first digest",
			imgRef: "ghcr.io/test/image:v1",
			payload: []payload.SimpleContainerImage{
				{
					Critical: payload.Critical{
						Image: payload.Image{
							DockerManifestDigest: "sha256:first",
						},
					},
				},
				{
					Critical: payload.Critical{
						Image: payload.Image{
							DockerManifestDigest: "sha256:second",
						},
					},
				},
			},
			wantDigest: "sha256:first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			digest, err := extractDigest(tt.imgRef, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractDigest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if digest != tt.wantDigest {
				t.Errorf("extractDigest() = %q, want %q", digest, tt.wantDigest)
			}
		})
	}
}

func TestCheckAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		payload     []payload.SimpleContainerImage
		annotations map[string]string
		wantErr     bool
	}{
		{
			name: "matching annotations",
			payload: []payload.SimpleContainerImage{
				{
					Optional: map[string]interface{}{
						"foo": "bar",
						"baz": "qux",
					},
				},
			},
			annotations: map[string]string{"foo": "bar"},
			wantErr:     false,
		},
		{
			name: "non-matching annotation value",
			payload: []payload.SimpleContainerImage{
				{
					Optional: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			annotations: map[string]string{"foo": "wrong"},
			wantErr:     true,
		},
		{
			name: "missing annotation key",
			payload: []payload.SimpleContainerImage{
				{
					Optional: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			annotations: map[string]string{"missing": "value"},
			wantErr:     true,
		},
		{
			name:        "nil annotations",
			payload:     []payload.SimpleContainerImage{{}},
			annotations: nil,
			wantErr:     false,
		},
		{
			name:        "empty annotations",
			payload:     []payload.SimpleContainerImage{{}},
			annotations: map[string]string{},
			wantErr:     false,
		},
		{
			name:        "empty payload with nil annotations",
			payload:     []payload.SimpleContainerImage{},
			annotations: nil,
			wantErr:     false,
		},
		{
			name: "multiple annotations all matching",
			payload: []payload.SimpleContainerImage{
				{
					Optional: map[string]interface{}{
						"a": "1",
						"b": "2",
						"c": "3",
					},
				},
			},
			annotations: map[string]string{"a": "1", "b": "2"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAnnotations(tt.payload, tt.annotations)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkAnnotations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStringToJSONMap(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		wantKey string
		wantVal interface{}
	}{
		{
			name:    "valid JSON string",
			input:   `{"key": "value"}`,
			wantErr: false,
			wantKey: "key",
			wantVal: "value",
		},
		{
			name:    "non-string input",
			input:   42,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   "not-json",
			wantErr: true,
		},
		{
			name:    "empty JSON object",
			input:   "{}",
			wantErr: false,
		},
		{
			name:    "nested JSON",
			input:   `{"outer": {"inner": true}}`,
			wantErr: false,
			wantKey: "outer",
		},
		{
			name:    "nil input",
			input:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := stringToJSONMap(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("stringToJSONMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.wantKey != "" {
				if _, ok := result[tt.wantKey]; !ok {
					t.Errorf("stringToJSONMap() result missing key %q", tt.wantKey)
				}
			}
		})
	}
}

func TestDecodePayload(t *testing.T) {
	// Create a valid in-toto statement with a SLSA predicate type
	statement := in_toto.Statement{ //nolint:staticcheck
		StatementHeader: in_toto.StatementHeader{ //nolint:staticcheck
			Type:          in_toto.StatementInTotoV01,
			PredicateType: "https://slsa.dev/provenance/v0.2",
			Subject: []in_toto.Subject{
				{
					Name:   "test-subject",
					Digest: map[string]string{"sha256": "abc123"},
				},
			},
		},
		Predicate: map[string]interface{}{
			"builder": map[string]interface{}{
				"id": "https://github.com/actions/runner",
			},
		},
	}

	statementBytes, err := json.Marshal(statement)
	if err != nil {
		t.Fatalf("failed to marshal statement: %v", err)
	}
	validPayload := base64.StdEncoding.EncodeToString(statementBytes)

	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{
			name:    "valid base64 encoded statement",
			payload: validPayload,
			wantErr: false,
		},
		{
			name:    "invalid base64",
			payload: "not-base64!@#$%",
			wantErr: true,
		},
		{
			name:    "valid base64 but invalid JSON",
			payload: base64.StdEncoding.EncodeToString([]byte("not-json")),
			wantErr: true,
		},
		{
			name:    "empty string",
			payload: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodePayload(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodePayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result == nil {
				t.Error("decodePayload() returned nil for valid input")
			}
		})
	}
}

func TestDecodeCosignCustomProvenanceV01(t *testing.T) {
	tests := []struct {
		name    string
		stmt    in_toto.Statement //nolint:staticcheck
		wantErr bool
	}{
		{
			name: "valid custom provenance with JSON data",
			stmt: in_toto.Statement{ //nolint:staticcheck
				StatementHeader: in_toto.StatementHeader{ //nolint:staticcheck
					Type:          attestation.CosignCustomProvenanceV01,
					PredicateType: attestation.CosignCustomProvenanceV01,
					Subject: []in_toto.Subject{
						{
							Name:   "test",
							Digest: map[string]string{"sha256": "abc"},
						},
					},
				},
				Predicate: map[string]interface{}{
					"Data":      `{"buildConfig": "test"}`,
					"Timestamp": "2024-01-01T00:00:00Z",
				},
			},
			wantErr: false,
		},
		{
			name: "valid custom provenance with non-JSON data",
			stmt: in_toto.Statement{ //nolint:staticcheck
				StatementHeader: in_toto.StatementHeader{ //nolint:staticcheck
					Type:          attestation.CosignCustomProvenanceV01,
					PredicateType: attestation.CosignCustomProvenanceV01,
					Subject: []in_toto.Subject{
						{
							Name:   "test",
							Digest: map[string]string{"sha256": "abc"},
						},
					},
				},
				Predicate: map[string]interface{}{
					"Data":      "plain-string-not-json",
					"Timestamp": "2024-01-01T00:00:00Z",
				},
			},
			wantErr: false,
		},
		{
			name: "wrong predicate type",
			stmt: in_toto.Statement{ //nolint:staticcheck
				StatementHeader: in_toto.StatementHeader{ //nolint:staticcheck
					Type:          in_toto.StatementInTotoV01,
					PredicateType: "https://slsa.dev/provenance/v0.2",
				},
				Predicate: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "non-map predicate",
			stmt: in_toto.Statement{ //nolint:staticcheck
				StatementHeader: in_toto.StatementHeader{ //nolint:staticcheck
					Type:          attestation.CosignCustomProvenanceV01,
					PredicateType: attestation.CosignCustomProvenanceV01,
				},
				Predicate: "not-a-map",
			},
			wantErr: true,
		},
		{
			name: "missing Data field in predicate",
			stmt: in_toto.Statement{ //nolint:staticcheck
				StatementHeader: in_toto.StatementHeader{ //nolint:staticcheck
					Type:          attestation.CosignCustomProvenanceV01,
					PredicateType: attestation.CosignCustomProvenanceV01,
					Subject: []in_toto.Subject{
						{
							Name:   "test",
							Digest: map[string]string{"sha256": "abc"},
						},
					},
				},
				Predicate: map[string]interface{}{
					"Timestamp": "2024-01-01T00:00:00Z",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeCosignCustomProvenanceV01(tt.stmt)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeCosignCustomProvenanceV01() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeStatements(t *testing.T) {
	tests := []struct {
		name      string
		sigs      []testSignature
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty signatures",
			sigs:      []testSignature{},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert testSignature to oci.Signature for the empty case
			stmts, _, err := decodeStatements(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeStatements() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(stmts) != tt.wantCount {
				t.Errorf("decodeStatements() returned %d statements, want %d", len(stmts), tt.wantCount)
			}
		})
	}
}

func TestBuildVerifyOptions(t *testing.T) {
	tests := []struct {
		name       string
		opts       images.Options
		wantLength int
	}{
		{
			name:       "both tlog and sct enabled",
			opts:       images.Options{IgnoreTlog: false, IgnoreSCT: false},
			wantLength: 2,
		},
		{
			name:       "tlog ignored",
			opts:       images.Options{IgnoreTlog: true, IgnoreSCT: false},
			wantLength: 1,
		},
		{
			name:       "sct ignored",
			opts:       images.Options{IgnoreTlog: false, IgnoreSCT: true},
			wantLength: 1,
		},
		{
			name:       "both ignored",
			opts:       images.Options{IgnoreTlog: true, IgnoreSCT: true},
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildVerifyOptions(tt.opts)
			if len(result) != tt.wantLength {
				t.Errorf("buildVerifyOptions() returned %d options, want %d", len(result), tt.wantLength)
			}
		})
	}
}

func TestDecodeStatementsFromBundles(t *testing.T) {
	tests := []struct {
		name    string
		bundles []*VerificationResult
		wantLen int
		wantErr bool
	}{
		{
			name:    "nil bundles",
			bundles: nil,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "empty bundles",
			bundles: []*VerificationResult{},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeStatementsFromBundles(tt.bundles)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeStatementsFromBundles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(result) != tt.wantLen {
				t.Errorf("decodeStatementsFromBundles() returned %d statements, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestNewCosignVerifier(t *testing.T) {
	v := NewVerifier()
	if v == nil {
		t.Fatal("NewVerifier() returned nil")
	}

	var _ images.ImageVerifier = v
}

func TestSignatureAlgorithmMap(t *testing.T) {
	tests := []struct {
		name     string
		algo     string
		expected crypto.Hash
		exists   bool
	}{
		{"empty default", "", crypto.SHA256, true},
		{"sha224", "sha224", crypto.SHA224, true},
		{"sha256", "sha256", crypto.SHA256, true},
		{"sha384", "sha384", crypto.SHA384, true},
		{"sha512", "sha512", crypto.SHA512, true},
		{"invalid", "md5", crypto.Hash(0), false},
		{"sha1 not supported", "sha1", crypto.Hash(0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, ok := signatureAlgorithmMap[tt.algo]
			if ok != tt.exists {
				t.Errorf("signatureAlgorithmMap[%q] exists = %v, want %v", tt.algo, ok, tt.exists)
			}
			if ok && hash != tt.expected {
				t.Errorf("signatureAlgorithmMap[%q] = %v, want %v", tt.algo, hash, tt.expected)
			}
		})
	}
}

func TestLoadCertPool(t *testing.T) {
	validPEM := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2wpSek9WFsKz
rhOCGhLMMRDnZWSDodcoek0JplQ2qooCIHtgr6UPTlFJCI0wTnsEnUayCkAyJHs5
0VnHSFJEljhX
-----END CERTIFICATE-----`

	tests := []struct {
		name    string
		roots   []byte
		wantErr bool
	}{
		{
			name:    "valid PEM certificate",
			roots:   []byte(validPEM),
			wantErr: false,
		},
		{
			name:    "invalid PEM data",
			roots:   []byte("not-a-certificate"),
			wantErr: true,
		},
		{
			name:    "empty data",
			roots:   []byte(""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := loadCertPool(tt.roots)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadCertPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && pool == nil {
				t.Error("loadCertPool() returned nil pool for valid input")
			}
		})
	}
}

func TestLoadCert(t *testing.T) {
	validPEM := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2wpSek9WFsKz
rhOCGhLMMRDnZWSDodcoek0JplQ2qooCIHtgr6UPTlFJCI0wTnsEnUayCkAyJHs5
0VnHSFJEljhX
-----END CERTIFICATE-----`

	tests := []struct {
		name    string
		pem     []byte
		wantErr bool
	}{
		{
			name:    "valid PEM certificate",
			pem:     []byte(validPEM),
			wantErr: false,
		},
		{
			name:    "invalid data",
			pem:     []byte("not-a-certificate"),
			wantErr: true,
		},
		{
			name:    "valid base64 of invalid cert",
			pem:     []byte(base64.StdEncoding.EncodeToString([]byte("not-a-cert"))),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := loadCert(tt.pem)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadCert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cert == nil {
				t.Error("loadCert() returned nil cert for valid input")
			}
		})
	}
}

func TestExtractCertExtensionValue(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "invalid extension key",
			key:     "invalidExtension",
			wantErr: true,
		},
		{
			name:    "empty extension key",
			key:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a zero-value CertExtensions (no cert)
			ce := cosignPkg.CertExtensions{}
			_, err := extractCertExtensionValue(tt.key, ce)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractCertExtensionValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
