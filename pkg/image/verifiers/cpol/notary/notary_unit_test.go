package notary

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/notaryproject/notation-core-go/signature"
	notation "github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/verifier/trustpolicy"
	"gotest.tools/v3/assert"
)

func TestCombineCerts(t *testing.T) {
	tests := []struct {
		name     string
		opts     verifiers.Options
		expected string
	}{
		{
			name:     "cert only",
			opts:     verifiers.Options{Cert: "cert-data"},
			expected: "cert-data",
		},
		{
			name:     "cert chain only",
			opts:     verifiers.Options{CertChain: "chain-data"},
			expected: "chain-data",
		},
		{
			name:     "both cert and chain",
			opts:     verifiers.Options{Cert: "cert-data", CertChain: "chain-data"},
			expected: "cert-data\nchain-data",
		},
		{
			name:     "neither cert nor chain",
			opts:     verifiers.Options{},
			expected: "",
		},
		{
			name:     "empty cert with chain",
			opts:     verifiers.Options{Cert: "", CertChain: "chain-data"},
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
	var _ verifiers.ImageVerifier = v
}

func TestNewVerifierType(t *testing.T) {
	v := NewVerifier()
	_, ok := v.(*notaryVerifier)
	if !ok {
		t.Fatal("NewVerifier() did not return a *notaryVerifier")
	}
}

func TestCombineCertsMultilineCerts(t *testing.T) {
	certPEM := `-----BEGIN CERTIFICATE-----
MIIB+jCCAaCgAwIBAgIUTest
-----END CERTIFICATE-----`
	chainPEM := `-----BEGIN CERTIFICATE-----
MIIB+jCCAaCgAwIBAgIUChain
-----END CERTIFICATE-----`

	opts := verifiers.Options{
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

// contextOnlyClient is a verifiers.Client that contributes nothing but the
// caller's context, so a test registry needs no credentials or TLS plumbing.
type contextOnlyClient struct{}

// Options returns the caller's context and nothing else.
func (contextOnlyClient) Options(ctx context.Context) ([]remote.Option, []name.Option, error) {
	return []remote.Option{remote.WithContext(ctx)}, nil, nil
}

// NameOptions returns no name options, so references are parsed as written.
func (contextOnlyClient) NameOptions() []name.Option { return nil }

// TestVerifyAttestatorsHonoursCallContext asserts verifyAttestators hands its
// own context to notation rather than a fresh one, so cancelling the admission
// request stops the registry round trips notation makes on our behalf. The test
// registry answers the manifest lookups, then cancels the context and never
// answers the signature listing, so the call can only return by honouring it.
func TestVerifyAttestatorsHonoursCallContext(t *testing.T) {
	manifest := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"layers":[]}`)
	sum := sha256.Sum256(manifest)
	dgst := "sha256:" + hex.EncodeToString(sum[:])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/referrers/"):
			cancel()
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
		case strings.HasSuffix(r.URL.Path, "/manifests/"+dgst):
			w.Header().Set("Content-Type", string(types.OCIManifestSchema1))
			w.Header().Set("Docker-Content-Digest", dgst)
			w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
			_, _ = w.Write(manifest)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	// httptest listens on 127.0.0.1, which go-containerregistry resolves over
	// plain http, so no TLS plumbing is needed here.
	ref, err := name.ParseReference(strings.TrimPrefix(srv.URL, "http://") + "/test/image:signed")
	assert.NilError(t, err)
	hash, err := v1.NewHash(dgst)
	assert.NilError(t, err)

	opts := verifiers.Options{ImageRef: ref.Name(), Cert: cert, Client: contextOnlyClient{}}
	_, err = verifyAttestators(ctx, &notaryVerifier{}, ref, opts, v1.Descriptor{Digest: hash})
	assert.ErrorContains(t, err, context.Canceled.Error())
}
