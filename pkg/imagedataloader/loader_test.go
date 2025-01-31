package imagedataloader

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
)

func Test_ImageDataLoader(t *testing.T) {
	idf, err := New(nil)
	assert.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), "ghcr.io/kyverno/kyverno:latest")
	assert.NoError(t, err)

	assert.Equal(t, img.Image, "ghcr.io/kyverno/kyverno:latest")
	assert.Equal(t, img.Registry, "ghcr.io")
	assert.Equal(t, img.Repository, "kyverno/kyverno")
	assert.Equal(t, img.Tag, "latest")
	assert.True(t, strings.HasPrefix(img.Digest, "sha256:"))
	assert.True(t, strings.HasPrefix(img.ResolvedImage, "ghcr.io/kyverno/kyverno:latest@sha256:"))

	indexMediaType := img.ImageIndex.(map[string]interface{})["mediaType"].(string)
	assert.Equal(t, indexMediaType, string(types.OCIImageIndex))

	fmt.Println(img.ConfigData)
	_, ok := img.ConfigData.(map[string]interface{})["architecture"]
	assert.True(t, ok)

	manifestMediaType := img.Manifest.(map[string]interface{})["mediaType"].(string)
	assert.Equal(t, manifestMediaType, string(types.OCIManifestSchema1))
}
