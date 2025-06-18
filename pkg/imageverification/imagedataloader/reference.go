package imagedataloader

import (
	"github.com/google/go-containerregistry/pkg/name"
)

type ImageReference struct {
	Image         string `json:"image,omitempty"`
	ResolvedImage string `json:"resolvedImage,omitempty"`
	Registry      string `json:"registry,omitempty"`
	Repository    string `json:"repository,omitempty"`
	Identifier    string `json:"identifier,omitempty"`
	Tag           string `json:"tag,omitempty"`
	Digest        string `json:"digest,omitempty"`
}

func ParseImageReference(image string, options ...Option) (ImageReference, error) {
	ref, err := name.ParseReference(image, nameOptions(options...)...)
	if err != nil {
		return ImageReference{}, err
	}
	img := ImageReference{
		Image:      image,
		Registry:   ref.Context().RegistryStr(),
		Repository: ref.Context().RepositoryStr(),
		Identifier: ref.Identifier(),
	}
	if _, ok := ref.(name.Tag); ok {
		img.Tag = ref.Identifier()
	} else {
		img.Digest = ref.Identifier()
	}
	return img, nil
}
