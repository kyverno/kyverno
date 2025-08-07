package imagedataloader

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
)

var (
	image = "ghcr.io/kyverno/test-verify-image:signed"
	ctx   = context.Background()
)

func Test_ImageDataLoader(t *testing.T) {
	idf, err := New(nil)
	assert.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	assert.Equal(t, img.Image, image)
	assert.Equal(t, img.Registry, "ghcr.io")
	assert.Equal(t, img.Repository, "kyverno/test-verify-image")
	assert.Equal(t, img.Tag, "signed")
	assert.Equal(t, img.Digest, "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
	assert.Equal(t, img.ResolvedImage, "ghcr.io/kyverno/test-verify-image:signed@sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")

	img, err = idf.FetchImageData(context.TODO(), "ghcr.io/kyverno/kyverno:latest")
	assert.NoError(t, err)
	indexMediaType := img.ImageIndex.(map[string]any)["mediaType"].(string)
	assert.Equal(t, indexMediaType, string(types.OCIImageIndex))

	arch := img.ConfigData.Architecture
	assert.True(t, len(arch) > 0)

	manifestMediaType := img.Manifest.MediaType
	assert.Equal(t, manifestMediaType, types.OCIManifestSchema1)
}

func Test_Referrers(t *testing.T) {
	idf, err := New(nil)
	assert.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	refList, err := img.FetchReferrers("application/vnd.cncf.notary.signature")
	assert.NoError(t, err)
	assert.Equal(t, len(refList), 2)
	assert.Equal(t, refList[0].ArtifactType, "application/vnd.cncf.notary.signature")
	assert.Equal(t, string(refList[0].MediaType), "application/vnd.oci.image.manifest.v1+json")

	data, desc, err := img.FetchReferrerData(refList[0])
	assert.NoError(t, err)
	assert.True(t, len(data) > 0)
	assert.Equal(t, string(desc.MediaType), "application/jose+json")
}
