package imagedataloader

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type imagedatafetcher struct {
	// TODO: Add caching and prefetching

	lister         k8scorev1.SecretInterface
	defaultOptions []crane.Option
}

type Fetcher interface {
	FetchImageData(ctx context.Context, image string, options ...Option) (*ImageData, error)
}

func New(lister k8scorev1.SecretInterface, opts ...Option) (*imagedatafetcher, error) {
	craneOpts, err := makeDefaultOpts(lister, opts...)
	if err != nil {
		return nil, err
	}

	return &imagedatafetcher{
		lister:         lister,
		defaultOptions: craneOpts,
	}, nil
}

type ImageData struct {
	Image         string      `json:"image,omitempty"`
	ResolvedImage string      `json:"resolvedImage,omitempty"`
	Registry      string      `json:"registry,omitempty"`
	Repository    string      `json:"repository,omitempty"`
	Tag           string      `json:"tag,omitempty"`
	Digest        string      `json:"digest,omitempty"`
	ImageIndex    interface{} `json:"imageIndex,omitempty"`
	Manifest      interface{} `json:"manifest,omitempty"`
	ConfigData    interface{} `json:"configData,omitempty"`
}

func (i *imagedatafetcher) FetchImageData(ctx context.Context, image string, options ...Option) (*ImageData, error) {
	img := ImageData{
		Image: image,
	}

	craneOpts, err := i.craneOptions(ctx, i.lister, options...)
	if err != nil {
		return nil, err
	}

	nameOpts := nameOptions(options...)
	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	img.Registry = ref.Context().RegistryStr()
	img.Repository = ref.Context().RepositoryStr()

	if _, ok := ref.(name.Tag); ok {
		img.Tag = ref.Identifier()
	} else {
		img.Digest = ref.Identifier()
	}

	manifestBytes, err := crane.Manifest(image, craneOpts...)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(manifestBytes, &img.Manifest); err != nil {
		return nil, err
	}

	configBytes, err := crane.Config(image, craneOpts...)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configBytes, &img.ConfigData); err != nil {
		return nil, err
	}

	desc, err := crane.Get(image, craneOpts...)
	if err != nil {
		return nil, err
	}

	if len(img.Digest) == 0 {
		img.Digest = desc.Digest.String()
	}

	if len(img.Tag) > 0 {
		img.ResolvedImage = fmt.Sprintf("%s:%s@%s", ref.Context().Name(), img.Tag, img.Digest)
	} else {
		img.ResolvedImage = fmt.Sprintf("%s@%s", ref.Context().Name(), img.Digest)
	}

	// error returned means no image index
	imgIndex, _ := desc.ImageIndex()
	if imgIndex != nil {
		imgIndexBytes, err := imgIndex.RawManifest()
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(imgIndexBytes, &img.ImageIndex); err != nil {
			return nil, err
		}
	}

	return &img, nil
}

func (i *imagedatafetcher) craneOptions(ctx context.Context, lister k8scorev1.SecretInterface, options ...Option) ([]crane.Option, error) {
	var opts []crane.Option
	opts = append(opts, i.defaultOptions...)

	authOpts, err := makeAuthOptions(lister, options...)
	if err != nil {
		return nil, err
	}

	opts = append(opts, authOpts...)
	opts = append(opts, crane.WithContext(ctx))

	return opts, nil
}
