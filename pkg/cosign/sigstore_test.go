package cosign

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
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
