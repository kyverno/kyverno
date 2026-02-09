package notary

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	notationregistry "github.com/notaryproject/notation-go/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type repositoryClient struct {
	image *imagedataloader.ImageData
}

func NewRepository(image *imagedataloader.ImageData) notationregistry.Repository {
	return &repositoryClient{
		image: image,
	}
}

func (c *repositoryClient) Resolve(_ context.Context, img string) (ocispec.Descriptor, error) {
	return c.image.FetchReference(img)
}

func (c *repositoryClient) ListSignatures(ctx context.Context, desc ocispec.Descriptor, fn func(signatureManifests []ocispec.Descriptor) error) error {
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

func (c *repositoryClient) FetchSignatureBlob(ctx context.Context, desc ocispec.Descriptor) ([]byte, ocispec.Descriptor, error) {
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
