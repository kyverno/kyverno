package cosign

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"gotest.tools/assert"
)

func TestSigstoreBundleSignatureVerification(t *testing.T) {
	opts := images.Options{
		SigstoreBundle: true,
		ImageRef:       "ghcr.io/vishal-chdhry/artifact-attestation-example:artifact-attestation",
		Issuer:         "https://token.actions.githubusercontent.com",
		Subject:        "https://github.com/vishal-chdhry/artifact-attestation-example/.github/workflows/build-attested-image.yaml@refs/heads/main",
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)

	opts.Subject = "invalid"
	_, err = verifier.VerifySignature(context.TODO(), opts)
	assert.ErrorContains(t, err, "sigstore bundle verification failed: no matching signatures found")
}

func TestSigstoreBundleSignatureResponse(t *testing.T) {
	opts := images.Options{
		SigstoreBundle: true,
		ImageRef:       "ghcr.io/vishal-chdhry/artifact-attestation-example:artifact-attestation",
		Issuer:         "https://token.actions.githubusercontent.com",
		Subject:        "https://github.com/vishal-chdhry/artifact-attestation-example/.github/workflows/build-attested-image.yaml@refs/heads/main",
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	response, err := verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)

	nameOpts := rc.NameOptions()
	ref, err := name.ParseReference(opts.ImageRef, nameOpts...)
	assert.NilError(t, err)

	desc, err := remote.Head(ref)
	assert.NilError(t, err)
	assert.Equal(t, desc.Digest.String(), response.Digest)
	assert.Equal(t, len(response.Statements), 0)
}

func TestSigstoreBundleAttestation(t *testing.T) {
	opts := images.Options{
		SigstoreBundle: true,
		ImageRef:       "ghcr.io/vishal-chdhry/artifact-attestation-example:artifact-attestation",
		Issuer:         "https://token.actions.githubusercontent.com",
		Subject:        "https://github.com/vishal-chdhry/artifact-attestation-example/.github/workflows/build-attested-image.yaml@refs/heads/main",
		Type:           "https://slsa.dev/provenance/v1",
	}

	rc, err := registryclient.New()
	assert.NilError(t, err)
	opts.Client = rc

	verifier := &cosignVerifier{}
	response, err := verifier.FetchAttestations(context.TODO(), opts)
	assert.NilError(t, err)

	nameOpts := rc.NameOptions()
	ref, err := name.ParseReference(opts.ImageRef, nameOpts...)
	assert.NilError(t, err)

	desc, err := remote.Head(ref)
	assert.NilError(t, err)

	assert.Equal(t, desc.Digest.String(), response.Digest)
	assert.Assert(t, len(response.Statements) > 0)

	buildType, ok := response.Statements[0]["predicate"].(map[string]interface{})["buildDefinition"].(map[string]interface{})["buildType"].(string)
	assert.Assert(t, ok)
	assert.Equal(t, buildType, "https://actions.github.io/buildtypes/workflow/v1")
}

func TestIssue_StaticKeyWithSigstoreBundle(t *testing.T) {
	desc := &v1.Descriptor{
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}
	opts := images.Options{
		SigstoreBundle: true,
		ImageRef:       "myregistry/path/myimage:mytag",
		Key:            "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...\n-----END PUBLIC KEY-----",
		Issuer:         "",
		Subject:        "",
		IssuerRegExp:   "",
		SubjectRegExp:  "",
	}

	_, err := buildPolicy(desc, opts)
	assert.NilError(t, err)
}

func TestIssue_StaticKeyNoTlogUpload(t *testing.T) {
	desc := v1.Descriptor{
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	opts := images.Options{
		SigstoreBundle: true,
		ImageRef:       "test/image:tag",
		Key:            "some-public-key",
		IgnoreTlog:     true,
	}

	_, err := buildPolicy(&desc, opts)
	assert.NilError(t, err)
}

func TestIssue_AllVerificationTypes(t *testing.T) {
	desc := &v1.Descriptor{
		Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	testCases := []struct {
		name        string
		opts        images.Options
		shouldWork  bool
		description string
	}{
		{
			name: "static_key_no_identity",
			opts: images.Options{
				SigstoreBundle: true,
				ImageRef:       "test:1",
				Key:            "key",
			},
			shouldWork:  true,
			description: "static key without identity fields",
		},
		{
			name: "keyless_with_issuer_and_subject",
			opts: images.Options{
				SigstoreBundle: true,
				ImageRef:       "test:2",
				Issuer:         "https://issuer.example.com",
				Subject:        "user@example.com",
			},
			shouldWork:  true,
			description: "keyless with both issuer and subject: should add cert identity",
		},
		{
			name: "keyless_with_regexp",
			opts: images.Options{
				SigstoreBundle: true,
				ImageRef:       "test:3",
				IssuerRegExp:   "https://.*",
				SubjectRegExp:  ".*@example.com",
			},
			shouldWork:  true,
			description: "keyless with regexp patterns: should add cert identity",
		},
		{
			name: "mixed_issuer_subject_regexp",
			opts: images.Options{
				SigstoreBundle: true,
				ImageRef:       "test:4",
				Issuer:         "https://issuer.example.com",
				SubjectRegExp:  ".*@example.com",
			},
			shouldWork:  true,
			description: "mixed exact and regexp: should add cert identity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log(tc.description)
			_, err := buildPolicy(desc, tc.opts)
			if tc.shouldWork {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
			}
		})
	}
}
