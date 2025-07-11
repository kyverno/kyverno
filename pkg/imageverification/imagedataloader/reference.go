package imagedataloader

import (
	"strings"

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
	
	if digest, ok := ref.(name.Digest); ok {
		img.Digest = digest.DigestStr()
		if colonIdx := strings.Index(image, ":"); colonIdx != -1 {
			if atIdx := strings.Index(image, "@"); atIdx != -1 && colonIdx < atIdx {
				imageWithoutDigest := image[:atIdx]
				if tagOnlyRef, err := name.ParseReference(imageWithoutDigest, nameOptions(options...)...); err == nil {
					if tag, ok := tagOnlyRef.(name.Tag); ok {
						img.Tag = tag.TagStr()
					}
				}
			}
		}
	} else if tag, ok := ref.(name.Tag); ok {
		img.Tag = tag.TagStr()
	}
	return img, nil
}
