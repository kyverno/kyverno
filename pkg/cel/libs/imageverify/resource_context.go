package imageverify

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
)

type resourceImageContextFactory interface {
	NewResourceImageContext() (imagedataloader.ImageContext, error)
}

type imageContextWithResourceFactory struct {
	imagedataloader.ImageContext
	newResource func() (imagedataloader.ImageContext, error)
}

func WithResourceImageContextFactory(images imagedataloader.ImageContext, newResource func() (imagedataloader.ImageContext, error)) imagedataloader.ImageContext {
	return &imageContextWithResourceFactory{ImageContext: images, newResource: newResource}
}

func (images *imageContextWithResourceFactory) NewResourceImageContext() (imagedataloader.ImageContext, error) {
	if images.newResource == nil {
		return nil, fmt.Errorf("resource image context factory is not configured")
	}
	return images.newResource()
}

func newResourceImageContext(images imagedataloader.ImageContext) (imagedataloader.ImageContext, error) {
	if factory, ok := images.(resourceImageContextFactory); ok {
		return factory.NewResourceImageContext()
	}
	return nil, fmt.Errorf("image context %T does not support resource image isolation", images)
}

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
	scoped, ok := author.(*resourceImageContext)
	if !ok {
		scoped = &resourceImageContext{
			author:   author,
			images:   make(map[string]struct{}, len(images)),
			authOpts: authOpts,
			nameOpts: nameOpts,
		}
	}
	for _, image := range images {
		scoped.images[image] = struct{}{}
		if reference, err := name.ParseReference(image, scoped.nameOpts...); err == nil {
			scoped.images[reference.Name()] = struct{}{}
		}
	}
	return scoped, nil
}

func (scoped *resourceImageContext) isResourceImage(image string) bool {
	if _, found := scoped.images[image]; found {
		return true
	}
	reference, err := name.ParseReference(image, scoped.nameOpts...)
	if err != nil {
		return false
	}
	_, found := scoped.images[reference.Name()]
	return found
}

func (scoped *resourceImageContext) Get(ctx context.Context, image string, authOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	if err := scoped.ValidateResourceImage(ctx, image); err != nil {
		return nil, err
	}
	return scoped.imageDataForValidatedImage(ctx, image, authOpts, nameOpts)
}

func (scoped *resourceImageContext) imageDataForValidatedImage(ctx context.Context, image string, authOpts []remote.Option, nameOpts []name.Option) (*imagedataloader.ImageData, error) {
	if scoped.isResourceImage(image) {
		if scoped.resource == nil {
			resource, err := newResourceImageContext(scoped.author)
			if err != nil {
				return nil, err
			}
			if resource == nil {
				return nil, fmt.Errorf("resource image context factory returned nil")
			}
			scoped.resource = resource
		}
		return scoped.resource.Get(ctx, image, scoped.authOpts, scoped.nameOpts)
	}
	return scoped.author.Get(ctx, image, authOpts, nameOpts)
}

func (scoped *resourceImageContext) ValidateResourceImage(ctx context.Context, image string) error {
	if scoped.isResourceImage(image) {
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
