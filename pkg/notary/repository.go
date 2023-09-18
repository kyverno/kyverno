package notary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	notationregistry "github.com/notaryproject/notation-go/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type repositoryClient struct {
	ref        name.Reference
	remoteOpts []remote.Option
}

func NewRepository(remoteOpts []remote.Option, ref name.Reference) notationregistry.Repository {
	return &repositoryClient{
		remoteOpts: remoteOpts,
		ref:        ref,
	}
}

func (c *repositoryClient) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	nameRef, err := name.ParseReference(c.getReferenceFromDigest(reference))
	if err != nil {
		return ocispec.Descriptor{}, nil
	}
	head, err := remote.Head(nameRef, c.remoteOpts...)
	if err != nil {
		return ocispec.Descriptor{}, nil
	}
	descriptor := v1ToOciSpecDescriptor(*head)
	return descriptor, nil
}

func (c *repositoryClient) ListSignatures(ctx context.Context, desc ocispec.Descriptor, fn func(signatureManifests []ocispec.Descriptor) error) error {
	referrers, err := remote.Referrers(c.ref.Context().Digest(desc.Digest.String()), c.remoteOpts...)
	if err != nil {
		return err
	}

	referrersDescs, err := referrers.IndexManifest()
	if err != nil {
		return err
	}

	// This check ensures that the manifest does not have an abnormal amount of referrers attached to it to protect against compromised images
	if len(referrersDescs.Manifests) > maxReferrersCount {
		return fmt.Errorf("failed to fetch referrers: to many referrers found, max limit is %d", maxReferrersCount)
	}

	descList := []ocispec.Descriptor{}
	for _, d := range referrersDescs.Manifests {
		if d.ArtifactType == notationregistry.ArtifactTypeNotation {
			descList = append(descList, v1ToOciSpecDescriptor(d))
		}
	}

	return fn(descList)
}

func (c *repositoryClient) FetchSignatureBlob(ctx context.Context, desc ocispec.Descriptor) ([]byte, ocispec.Descriptor, error) {
	manifestRef, err := name.ParseReference(c.getReferenceFromDescriptor(desc))
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	remoteDesc, err := remote.Get(manifestRef, c.remoteOpts...)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}
	manifestBytes, err := remoteDesc.RawManifest()
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, ocispec.Descriptor{}, err
	}
	manifestDesc := manifest.Layers[0]

	// This check ensures that the size of a layer isn't abnormally large to avoid malicious payloads
	if manifestDesc.Size > int64(maxPayloadSize) {
		return nil, ocispec.Descriptor{}, fmt.Errorf("payload size is too large, max size is %d: %+v", maxPayloadSize, manifestDesc)
	}

	signatureBlobRef, err := name.ParseReference(c.getReferenceFromDescriptor(manifestDesc))
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	signatureBlobLayer, err := remote.Layer(signatureBlobRef.Context().Digest(signatureBlobRef.Identifier()), c.remoteOpts...)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	io, err := signatureBlobLayer.Uncompressed()
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}
	SigBlobBuf := new(bytes.Buffer)

	_, err = SigBlobBuf.ReadFrom(io)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}
	return SigBlobBuf.Bytes(), manifestDesc, nil
}

func (c *repositoryClient) PushSignature(ctx context.Context, mediaType string, blob []byte, subject ocispec.Descriptor, annotations map[string]string) (blobDesc, manifestDesc ocispec.Descriptor, err error) {
	return ocispec.Descriptor{}, ocispec.Descriptor{}, fmt.Errorf("push signature is not implemented")
}

func v1ToOciSpecDescriptor(v1desc v1.Descriptor) ocispec.Descriptor {
	ociDesc := ocispec.Descriptor{
		MediaType:   string(v1desc.MediaType),
		Digest:      digest.Digest(v1desc.Digest.String()),
		Size:        v1desc.Size,
		URLs:        v1desc.URLs,
		Annotations: v1desc.Annotations,
		Data:        v1desc.Data,

		ArtifactType: v1desc.ArtifactType,
	}
	if v1desc.Platform != nil {
		ociDesc.Platform = &ocispec.Platform{
			Architecture: v1desc.Platform.Architecture,
			OS:           v1desc.Platform.OS,
			OSVersion:    v1desc.Platform.OSVersion,
		}
	}
	return ociDesc
}

func (c *repositoryClient) getReferenceFromDescriptor(desc ocispec.Descriptor) string {
	return GetReferenceFromDescriptor(desc, c.ref)
}

func (c *repositoryClient) getReferenceFromDigest(digest string) string {
	return c.ref.Context().RegistryStr() + "/" + c.ref.Context().RepositoryStr() + "@" + digest
}

func GetReferenceFromDescriptor(desc ocispec.Descriptor, ref name.Reference) string {
	return ref.Context().RegistryStr() + "/" + ref.Context().RepositoryStr() + "@" + desc.Digest.String()
}
