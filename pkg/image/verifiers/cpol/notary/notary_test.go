package notary

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/sdk/extensions/registryclient"
	godigest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

var cert = `-----BEGIN CERTIFICATE-----
MIIDTTCCAjWgAwIBAgIJAPI+zAzn4s0xMA0GCSqGSIb3DQEBCwUAMEwxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJXQTEQMA4GA1UEBwwHU2VhdHRsZTEPMA0GA1UECgwG
Tm90YXJ5MQ0wCwYDVQQDDAR0ZXN0MB4XDTIzMDUyMjIxMTUxOFoXDTMzMDUxOTIx
MTUxOFowTDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAldBMRAwDgYDVQQHDAdTZWF0
dGxlMQ8wDQYDVQQKDAZOb3RhcnkxDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDNhTwv+QMk7jEHufFfIFlBjn2NiJaYPgL4eBS+
b+o37ve5Zn9nzRppV6kGsa161r9s2KkLXmJrojNy6vo9a6g6RtZ3F6xKiWLUmbAL
hVTCfYw/2n7xNlVMjyyUpE+7e193PF8HfQrfDFxe2JnX5LHtGe+X9vdvo2l41R6m
Iia04DvpMdG4+da2tKPzXIuLUz/FDb6IODO3+qsqQLwEKmmUee+KX+3yw8I6G1y0
Vp0mnHfsfutlHeG8gazCDlzEsuD4QJ9BKeRf2Vrb0ywqNLkGCbcCWF2H5Q80Iq/f
ETVO9z88R7WheVdEjUB8UrY7ZMLdADM14IPhY2Y+tLaSzEVZAgMBAAGjMjAwMAkG
A1UdEwQCMAAwDgYDVR0PAQH/BAQDAgeAMBMGA1UdJQQMMAoGCCsGAQUFBwMDMA0G
CSqGSIb3DQEBCwUAA4IBAQBX7x4Ucre8AIUmXZ5PUK/zUBVOrZZzR1YE8w86J4X9
kYeTtlijf9i2LTZMfGuG0dEVFN4ae3CCpBst+ilhIndnoxTyzP+sNy4RCRQ2Y/k8
Zq235KIh7uucq96PL0qsF9s2RpTKXxyOGdtp9+HO0Ty5txJE2txtLDUIVPK5WNDF
ByCEQNhtHgN6V20b8KU2oLBZ9vyB8V010dQz0NRTDLhkcvJig00535/LUylECYAJ
5/jn6XKt6UYCQJbVNzBg/YPGc1RF4xdsGVDBben/JXpeGEmkdmXPILTKd9tZ5TC0
uOKpF5rWAruB5PCIrquamOejpXV9aQA/K2JQDuc0mcKz
-----END CERTIFICATE-----`

func TestExtractStatements(t *testing.T) {
	imageRef := "ghcr.io/kyverno/test-verify-image:signed"
	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(ref)
	assert.NilError(t, err)
	referrers, err := remote.Referrers(ref.Context().Digest(repoDesc.Digest.String()))
	assert.NilError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NilError(t, err)

	for _, referrer := range referrersDescs.Manifests {
		if referrer.ArtifactType == "application/vnd.cncf.notary.signature" {
			statements, err := extractStatements(context.Background(), ref, referrer, nil, nil)
			assert.NilError(t, err)
			assert.Assert(t, len(statements) == 1)
			assert.Assert(t, statements[0]["type"] == referrer.ArtifactType)
			assert.Assert(t, statements[0]["mediaType"] == string(referrer.MediaType))
		}
	}
}

func TestNotaryImageVerification(t *testing.T) {
	opts := verifiers.Options{
		ImageRef: "ghcr.io/kyverno/test-verify-image:signed",
		Cert:     cert,
	}

	rc := registryclient.New()
	opts.Client = rc

	verifier := &notaryVerifier{}
	_, err := verifier.VerifySignature(context.TODO(), opts)
	assert.NilError(t, err)
}

func TestExtractStatementRejectsOversizedPredicate(t *testing.T) {
	repo := newTestRegistry(t)
	restore := maxPayloadSize
	maxPayloadSize = 1 << 10
	t.Cleanup(func() { maxPayloadSize = restore })

	data := gzipBytes(t, int(maxPayloadSize)+1)
	layerDigest, layerSize := pushBlob(t, repo, data, "application/vnd.in-toto+json")
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeEmptyJSON},
		Layers: []ocispec.Descriptor{
			{MediaType: "application/vnd.in-toto+json", Digest: godigest.Digest(layerDigest.DigestStr()), Size: layerSize},
		},
	}
	manifestRef := pushManifest(t, repo, manifest)

	repoRef, err := name.ParseReference(repo.Name())
	assert.NilError(t, err)
	h, err := v1.NewHash(manifestRef.DigestStr())
	assert.NilError(t, err)

	_, err = extractStatement(ctx, repoRef, v1.Descriptor{Digest: h}, nil, nil)
	assert.ErrorContains(t, err, "exceeds")
}

// TestExtractStatementRoundTripsValidPredicate is a sanity control: a small predicate under
// the limit must still verify, proving the harness above fails for the right reason.
func TestExtractStatementRoundTripsValidPredicate(t *testing.T) {
	repo := newTestRegistry(t)
	predicate := []byte(`{"hello":"world"}`)
	layerDigest, layerSize := pushBlob(t, repo, predicate, "application/vnd.in-toto+json")
	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    ocispec.Descriptor{MediaType: ocispec.MediaTypeEmptyJSON},
		Layers: []ocispec.Descriptor{
			{MediaType: "application/vnd.in-toto+json", Digest: godigest.Digest(layerDigest.DigestStr()), Size: layerSize},
		},
	}
	manifestRef := pushManifest(t, repo, manifest)

	repoRef, err := name.ParseReference(repo.Name())
	assert.NilError(t, err)
	h, err := v1.NewHash(manifestRef.DigestStr())
	assert.NilError(t, err)

	data, err := extractStatement(ctx, repoRef, v1.Descriptor{Digest: h, ArtifactType: "test"}, nil, nil)
	assert.NilError(t, err)
	assert.Equal(t, data["type"], "test")
	predicateMap, ok := data["predicate"].(map[string]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, predicateMap["hello"], "world")
}
