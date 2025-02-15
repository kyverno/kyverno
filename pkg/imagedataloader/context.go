package imagedataloader

import (
	"context"

	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type imageContext struct {
	f    Fetcher
	list map[string]*ImageData
}

// ImageContext stores a list of imagedata, it lives as long as
// the admission request. Get request for images either returned a prefetched image or
// fetches it from the registry. It is used to share image data for a policy across policies
type ImageContext interface {
	AddImages(ctx context.Context, images []string, opts ...Option) error
	Get(ctx context.Context, image string, opts ...Option) (*ImageData, error)
}

func NewImageContext(lister k8scorev1.SecretInterface, opts ...Option) (ImageContext, error) {
	idl, err := New(lister, opts...)
	if err != nil {
		return nil, err
	}
	return &imageContext{
		f:    idl,
		list: make(map[string]*ImageData),
	}, nil
}

func (idc *imageContext) AddImages(ctx context.Context, images []string, opts ...Option) error {
	for _, img := range images {
		if _, found := idc.list[img]; found {
			continue
		}

		data, err := idc.f.FetchImageData(ctx, img, opts...)
		if err != nil {
			return err
		}
		idc.list[img] = data
	}
	return nil
}

func (idc *imageContext) Get(ctx context.Context, image string, opts ...Option) (*ImageData, error) {
	if data, found := idc.list[image]; found {
		return data, nil
	}

	data, err := idc.f.FetchImageData(ctx, image, opts...)
	if err != nil {
		return nil, err
	}
	idc.list[image] = data

	return data, nil
}
