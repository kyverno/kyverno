package notary

import (
	"fmt"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/pkg/images"
	notation "github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-core-go/signature"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
)

func TestCombineCerts(t *testing.T) {
	tests := []struct {
		name     string
		opts     images.Options
		expected string
	}{
		{
			name:     "cert only",
			opts:     images.Options{Cert: "cert-data"},
			expected: "cert-data",
		},
		{
			name:     "cert chain only",
			opts:     images.Options{CertChain: "chain-data"},
			expected: "chain-data",
		},
		{
			name:     "both cert and chain",
			opts:     images.Options{Cert: "cert-data", CertChain: "chain-data"},
			expected: "cert-data\nchain-data",
		},
		{
			name:     "neither cert nor chain",
			opts:     images.Options{},
			expected: "",
		},
		{
			name:     "empty cert with chain",
			opts:     images.Options{Cert: "", CertChain: "chain-data"},
			expected: "chain-data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineCerts(tt.opts)
			if result != tt.expected {
				t.Errorf("combineCerts() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMatchArtifactType(t *testing.T) {
	tests := []struct {
		name         string
		desc         v1.Descriptor
		expectedType string
		wantMatch    bool
		wantType     string
		wantErr      bool
	}{
		{
			name:         "matching artifact type",
			desc:         v1.Descriptor{ArtifactType: "application/vnd.cncf.notary.signature"},
			expectedType: "application/vnd.cncf.notary.signature",
			wantMatch:    true,
			wantType:     "application/vnd.cncf.notary.signature",
		},
		{
			name:         "non-matching artifact type",
			desc:         v1.Descriptor{ArtifactType: "application/vnd.cncf.notary.signature"},
			expectedType: "application/vnd.other.type",
			wantMatch:    false,
			wantType:     "",
		},
		{
			name:         "empty expected type",
			desc:         v1.Descriptor{ArtifactType: "application/vnd.cncf.notary.signature"},
			expectedType: "",
			wantMatch:    false,
			wantType:     "",
		},
		{
			name:         "both empty",
			desc:         v1.Descriptor{ArtifactType: ""},
			expectedType: "",
			wantMatch:    false,
			wantType:     "",
		},
		{
			name:         "empty descriptor artifact type with non-empty expected",
			desc:         v1.Descriptor{ArtifactType: ""},
			expectedType: "application/vnd.cncf.notary.signature",
			wantMatch:    false,
			wantType:     "",
		},
		{
			name: "matching with other descriptor fields set",
			desc: v1.Descriptor{
				ArtifactType: "application/vnd.cncf.notary.signature",
				MediaType:    types.OCIManifestSchema1,
				Size:         1024,
				Annotations:  map[string]string{"key": "value"},
			},
			expectedType: "application/vnd.cncf.notary.signature",
			wantMatch:    true,
			wantType:     "application/vnd.cncf.notary.signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, artifactType, err := matchArtifactType(tt.desc, tt.expectedType)
			if (err != nil) != tt.wantErr {
				t.Errorf("matchArtifactType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if match != tt.wantMatch {
				t.Errorf("matchArtifactType() match = %v, want %v", match, tt.wantMatch)
			}
			if artifactType != tt.wantType {
				t.Errorf("matchArtifactType() type = %q, want %q", artifactType, tt.wantType)
			}
		})
	}
}

func TestVerifyOutcomes(t *testing.T) {
	v := &notaryVerifier{}

	tests := []struct {
		name     string
		outcomes []*notation.VerificationOutcome
		wantErr  bool
		errCount int
	}{
		{
			name:     "nil outcomes",
			outcomes: nil,
			wantErr:  false,
		},
		{
			name:     "empty outcomes",
			outcomes: []*notation.VerificationOutcome{},
			wantErr:  false,
		},
		{
			name: "single successful outcome",
			outcomes: []*notation.VerificationOutcome{
				{
					EnvelopeContent: &signature.EnvelopeContent{
						Payload: signature.Payload{
							Content:     []byte(`{"test": true}`),
							ContentType: "application/json",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "single error outcome",
			outcomes: []*notation.VerificationOutcome{
				{
					Error: fmt.Errorf("signature verification failed"),
				},
			},
			wantErr: true,
		},
		{
			name: "multiple errors",
			outcomes: []*notation.VerificationOutcome{
				{
					Error: fmt.Errorf("error 1"),
				},
				{
					Error: fmt.Errorf("error 2"),
				},
			},
			wantErr: true,
		},
		{
			name: "mixed outcomes - error and success",
			outcomes: []*notation.VerificationOutcome{
				{
					Error: fmt.Errorf("error 1"),
				},
				{
					EnvelopeContent: &signature.EnvelopeContent{
						Payload: signature.Payload{
							Content:     []byte(`{"test": true}`),
							ContentType: "application/json",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.verifyOutcomes(tt.outcomes)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyOutcomes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildPolicy(t *testing.T) {
	v := &notaryVerifier{}
	policy := v.buildPolicy()

	if policy == nil {
		t.Fatal("buildPolicy() returned nil")
	}
	if policy.Version != "1.0" {
		t.Errorf("buildPolicy() version = %q, want %q", policy.Version, "1.0")
	}
	if len(policy.TrustPolicies) != 1 {
		t.Fatalf("buildPolicy() trust policies count = %d, want 1", len(policy.TrustPolicies))
	}

	tp := policy.TrustPolicies[0]
	if tp.Name != "kyverno" {
		t.Errorf("buildPolicy() policy name = %q, want %q", tp.Name, "kyverno")
	}
	if len(tp.RegistryScopes) != 1 || tp.RegistryScopes[0] != "*" {
		t.Errorf("buildPolicy() registry scopes = %v, want [\"*\"]", tp.RegistryScopes)
	}
	if tp.SignatureVerification.VerificationLevel != trustpolicy.LevelStrict.Name {
		t.Errorf("buildPolicy() verification level = %q, want %q", tp.SignatureVerification.VerificationLevel, trustpolicy.LevelStrict.Name)
	}
	if len(tp.TrustStores) != 1 || tp.TrustStores[0] != "ca:kyverno" {
		t.Errorf("buildPolicy() trust stores = %v, want [\"ca:kyverno\"]", tp.TrustStores)
	}
	if len(tp.TrustedIdentities) != 1 || tp.TrustedIdentities[0] != "*" {
		t.Errorf("buildPolicy() trusted identities = %v, want [\"*\"]", tp.TrustedIdentities)
	}
}

func TestNewVerifier(t *testing.T) {
	v := NewVerifier()
	if v == nil {
		t.Fatal("NewVerifier() returned nil")
	}

	// Verify it implements the ImageVerifier interface
	var _ images.ImageVerifier = v
}

func TestNewVerifierType(t *testing.T) {
	v := NewVerifier()
	nv, ok := v.(*notaryVerifier)
	if !ok {
		t.Fatal("NewVerifier() did not return a *notaryVerifier")
	}
	if nv.log.GetSink() == nil {
		t.Error("NewVerifier() logger sink is nil")
	}
}

func TestCombineCertsMultilineCerts(t *testing.T) {
	certPEM := `-----BEGIN CERTIFICATE-----
MIIB+jCCAaCgAwIBAgIUTest
-----END CERTIFICATE-----`
	chainPEM := `-----BEGIN CERTIFICATE-----
MIIB+jCCAaCgAwIBAgIUChain
-----END CERTIFICATE-----`

	opts := images.Options{
		Cert:      certPEM,
		CertChain: chainPEM,
	}

	result := combineCerts(opts)
	expected := certPEM + "\n" + chainPEM
	if result != expected {
		t.Errorf("combineCerts() with multiline PEM certs:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestMatchArtifactTypePartialMatch(t *testing.T) {
	// Verify that partial matches do not succeed (strict equality)
	desc := v1.Descriptor{ArtifactType: "application/vnd.cncf.notary.signature"}

	match, _, _ := matchArtifactType(desc, "application/vnd.cncf.notary")
	if match {
		t.Error("matchArtifactType() should not match partial artifact types")
	}

	match, _, _ = matchArtifactType(desc, "application/vnd.cncf.notary.signature.extra")
	if match {
		t.Error("matchArtifactType() should not match extended artifact types")
	}
}

func TestVerifyOutcomesErrorMessages(t *testing.T) {
	v := &notaryVerifier{}

	outcomes := []*notation.VerificationOutcome{
		{Error: fmt.Errorf("first error")},
		{Error: fmt.Errorf("second error")},
	}

	err := v.verifyOutcomes(outcomes)
	if err == nil {
		t.Fatal("verifyOutcomes() expected error, got nil")
	}

	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("verifyOutcomes() error message should not be empty")
	}
}
