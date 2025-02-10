package imagedataloader

import (
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
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
