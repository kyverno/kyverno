package notary

import (
	"context"
	"fmt"

	"github.com/kyverno/sdk/extensions/imagedataloader"
	notationregistry "github.com/notaryproject/notation-go/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type repositoryClient struct {
	image *imagedataloader.ImageData
}

// NewRepository adapts ImageData to the Notation registry interface.
func NewRepository(image *imagedataloader.ImageData) notationregistry.Repository {
	return &repositoryClient{
		image: image,
	}
}

// The ImageData calls below don't take a context, so each method checks ctx
// first to stop a verification that was cancelled because an earlier-listed
// referrer verified.

// Resolve fetches an image descriptor unless the verification context was cancelled.
func (c *repositoryClient) Resolve(ctx context.Context, img string) (ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	return c.image.FetchReference(img)
}

// ListSignatures passes the image's Notation referrer descriptors to fn.
func (c *repositoryClient) ListSignatures(ctx context.Context, desc ocispec.Descriptor, fn func(signatureManifests []ocispec.Descriptor) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	gcrDesc, err := c.image.FetchReferrersForDigest(desc.Digest.String(), notationregistry.ArtifactTypeNotation)
	if err != nil {
		return err
	}

	descriptorList := make([]ocispec.Descriptor, 0, len(gcrDesc))
	for _, d := range gcrDesc {
		descriptorList = append(descriptorList, imagedataloader.GCRtoOCISpecDesc(d))
	}

	return fn(descriptorList)
}

// FetchSignatureBlob retrieves a referrer's payload unless the context was cancelled.
func (c *repositoryClient) FetchSignatureBlob(ctx context.Context, desc ocispec.Descriptor) ([]byte, ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, ocispec.Descriptor{}, err
	}
	gcrDesc, err := imagedataloader.OCISpectoGCRDesc(desc)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	data, layerDesc, err := c.image.FetchReferrerData(*gcrDesc)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	return data, imagedataloader.GCRtoOCISpecDesc(*layerDesc), nil
}

func (c *repositoryClient) PushSignature(ctx context.Context, mediaType string, blob []byte, subject ocispec.Descriptor, annotations map[string]string) (blobDesc, manifestDesc ocispec.Descriptor, err error) {
	return ocispec.Descriptor{}, ocispec.Descriptor{}, fmt.Errorf("push signature is not implemented")
}
