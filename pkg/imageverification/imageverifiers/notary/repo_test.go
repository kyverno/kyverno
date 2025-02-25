package notary

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	notationregistry "github.com/notaryproject/notation-go/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

var (
	image = "ghcr.io/kyverno/test-verify-image:signed"
	ctx   = context.Background()
)

func TestResolve(t *testing.T) {
	repositoryClient, img := setuprepo(t)

	desc, err := repositoryClient.Resolve(ctx, img.Digest)
	assert.NoError(t, err)
	assert.Equal(t, desc.Digest.String(), "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
	assert.Equal(t, desc.MediaType, "application/vnd.docker.distribution.manifest.v2+json")
}

func TestListSignatures(t *testing.T) {
	repositoryClient, img := setuprepo(t)
	sigs := 0

	fn := func(l []ocispec.Descriptor) error {
		sigs = len(l)
		return nil
	}

	err := repositoryClient.ListSignatures(ctx, ocispec.Descriptor{Digest: digest.Digest(img.Digest)}, fn)
	assert.NoError(t, err)
	assert.Equal(t, sigs, 2)
}

func TestFetchSignatureBlob(t *testing.T) {
	repositoryClient, img := setuprepo(t)
	ref, err := name.ParseReference(image)
	assert.NoError(t, err)

	referrers, err := remote.Referrers(ref.Context().Digest(img.Digest))
	assert.NoError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NoError(t, err)

	for _, d := range referrersDescs.Manifests {
		if d.ArtifactType == notationregistry.ArtifactTypeNotation {
			_, desc, err := repositoryClient.FetchSignatureBlob(ctx, imagedataloader.GCRtoOCISpecDesc(d))
			assert.NoError(t, err)
			assert.Equal(t, desc.MediaType, "application/jose+json")
		}
	}
}

func setuprepo(t *testing.T) (notationregistry.Repository, *imagedataloader.ImageData) {
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(ctx, image)
	assert.NoError(t, err)
	return NewRepository(img), img
}
