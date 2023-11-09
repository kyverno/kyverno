package notary

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	notationregistry "github.com/notaryproject/notation-go/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/assert"
)

var (
	imageRef = "jimnotarytest.azurecr.io/jim/net-monitor:v1"
	ctx      = context.Background()
)

func TestResolve(t *testing.T) {
	nameRef, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(nameRef)
	assert.NilError(t, err)

	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)

	repositoryClient := NewRepository(nil, ref)

	desc, err := repositoryClient.Resolve(ctx, repoDesc.Digest.String())
	assert.NilError(t, err)
	assert.Equal(t, desc.Digest.String(), "sha256:ba7000206594c2d72c3ab550453004c0dc50961157e5ebd2fb8ea1890099d02d")
	assert.Equal(t, desc.MediaType, "application/vnd.docker.distribution.manifest.v2+json")
}

func TestListSignatures(t *testing.T) {
	nameRef, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(nameRef)
	assert.NilError(t, err)

	ociDesc := v1ToOciSpecDescriptor(*repoDesc)
	assert.Equal(t, ociDesc.Digest.String(), repoDesc.Digest.String())

	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)

	repositoryClient := NewRepository(nil, ref)
	fn := func(_ []ocispec.Descriptor) error {
		return nil
	}

	err = repositoryClient.ListSignatures(ctx, ociDesc, fn)
	assert.NilError(t, err)
}

func TestFetchSignatureBlob(t *testing.T) {
	nameRef, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := remote.Head(nameRef)
	assert.NilError(t, err)

	ociDesc := v1ToOciSpecDescriptor(*repoDesc)
	assert.Equal(t, ociDesc.Digest.String(), repoDesc.Digest.String())

	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)

	repositoryClient := NewRepository(nil, ref)

	referrers, err := remote.Referrers(ref.Context().Digest(ociDesc.Digest.String()))
	assert.NilError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NilError(t, err)

	for _, d := range referrersDescs.Manifests {
		if d.ArtifactType == notationregistry.ArtifactTypeNotation {
			_, desc, err := repositoryClient.FetchSignatureBlob(ctx, v1ToOciSpecDescriptor(d))
			assert.NilError(t, err)
			assert.Equal(t, desc.MediaType, "application/jose+json")
			assert.Equal(t, desc.Digest.String(), "sha256:746134b09f89451497668c598857d87ca660bb3d0b888832235c460d8d2697f3")
		}
	}
}
