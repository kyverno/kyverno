package imagedataloader

import (
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func GCRtoOCISpecDesc(v1desc gcrv1.Descriptor) ocispec.Descriptor {
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

func OCISpectoGCRDesc(ocidesc ocispec.Descriptor) (*gcrv1.Descriptor, error) {
	gcrDesc := &gcrv1.Descriptor{
		MediaType:    types.MediaType(ocidesc.MediaType),
		Size:         ocidesc.Size,
		URLs:         ocidesc.URLs,
		Annotations:  ocidesc.Annotations,
		Data:         ocidesc.Data,
		ArtifactType: ocidesc.ArtifactType,
	}

	digest, err := gcrv1.NewHash(ocidesc.Digest.String())
	if err != nil {
		return nil, err
	}

	gcrDesc.Digest = digest
	if ocidesc.Platform != nil {
		gcrDesc.Platform = &gcrv1.Platform{
			Architecture: ocidesc.Platform.Architecture,
			OS:           ocidesc.Platform.OS,
			OSVersion:    ocidesc.Platform.OSVersion,
		}
	}

	return gcrDesc, nil
}

func BuildRemoteOpts(secrets []string, providers []string, insecure bool) []Option {
	opts := make([]Option, 0)

	if insecure {
		opts = append(opts, WithInsecure(insecure))
	}

	if len(providers) != 0 {
		opts = append(opts, WithCredentialProviders(providers...))
	}

	if len(secrets) != 0 {
		opts = append(opts, WithPullSecret(secrets))
	}

	return opts
}
