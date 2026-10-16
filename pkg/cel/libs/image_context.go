package libs

import (
	"context"
	"errors"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
)

type imageDataFunc func(string, []remote.Option) (map[string]any, error)

func (f imageDataFunc) GetImageData(image string, options []remote.Option) (map[string]any, error) {
	return f(image, options)
}

// ImageDataContext binds the image CEL receiver to one evaluation's context.
// The shared provider and compiled CEL programs remain unchanged and can be
// reused concurrently. Providers without the context extension retain their
// existing behavior, including CLI fixtures.
func ImageDataContext(ctx context.Context, provider imagedata.ContextInterface) imagedata.Context {
	if provider == nil {
		provider = LibraryContext
	}
	return imagedata.Context{ContextInterface: imageDataFunc(func(image string, options []remote.Option) (map[string]any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if provider == nil {
			return nil, errors.New("image metadata context is not configured")
		}
		if contextual, ok := provider.(interface {
			GetImageDataWithContext(context.Context, string, []remote.Option) (map[string]any, error)
		}); ok {
			return contextual.GetImageDataWithContext(ctx, image, options)
		}
		return provider.GetImageData(image, options)
	})}
}
