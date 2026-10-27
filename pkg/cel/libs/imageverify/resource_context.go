package imageverify

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
)

type resourceImageContext struct {
	author   imagedataloader.ImageContext
	resource imagedataloader.ImageContext
	images   map[string]struct{}
	authOpts []remote.Option
	nameOpts []name.Option
}

// NewResourceImageContext isolates workload image data from author-populated caches.
// Only extracted resource references use the guarded options, including later
// signature and referrer reads made through the resulting ImageData.
func NewResourceImageContext(author imagedataloader.ImageContext, images []string, authOpts []remote.Option, nameOpts []name.Option) (imagedataloader.ImageContext, error) {
	if len(images) == 0 {
		return author, nil
	}
	if scoped, ok := author.(*resourceImageContext); ok {
		for _, image := range images {
			scoped.images[image] = struct{}{}
		}
		return scoped, nil
	}
	resource, err := imagedataloader.NewImageContext(nil, nil, nil)
	if err != nil {
		return nil, err
	}
	refs := make(map[string]struct{}, len(images))
	for _, image := range images {
		refs[image] = struct{}{}
	}
	return &resourceImageContext{author: author, resource: resource, images: refs, authOpts: authOpts, nameOpts: nameOpts}, nil
}

func (scoped *resourceImageContext) Get(ctx context.Context, image string, authOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	if _, resource := scoped.images[image]; resource {
		if err := scoped.ValidateResourceImage(ctx, image); err != nil {
			return nil, err
		}
		return scoped.resource.Get(ctx, image, scoped.authOpts, scoped.nameOpts)
	}
	return scoped.author.Get(ctx, image, authOpts, nameOpts)
}

func (scoped *resourceImageContext) ValidateResourceImage(ctx context.Context, image string) error {
	if _, resource := scoped.images[image]; resource {
		return registryclient.ValidateResourceImage(ctx, image, scoped.nameOpts...)
	}
	return nil
}

func (scoped *resourceImageContext) AddImages(ctx context.Context, images []string, authOpts []remote.Option, nameOpts []name.Option) error {
	for _, image := range images {
		if _, err := scoped.Get(ctx, image, authOpts, nameOpts); err != nil {
			return err
		}
	}
	return nil
}
