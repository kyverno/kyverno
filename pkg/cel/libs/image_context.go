package libs

import (
	"context"
	"errors"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type imageDataFunc func(string, []remote.Option) (map[string]any, error)

type imageKey struct {
	image string
	nopts int
	opts  *remote.Option
}

type imageResult struct {
	once sync.Once
	data map[string]any
	err  error
}

func (f imageDataFunc) GetImageData(image string, options []remote.Option) (map[string]any, error) {
	return f(image, options)
}

// ImageDataContext binds the image CEL receiver to one evaluation's context.
// The shared provider and compiled CEL programs remain unchanged and can be
// reused concurrently. Providers without the context extension retain their
// existing behavior, including CLI fixtures.
func ImageDataContext(ctx context.Context, provider imagedata.ContextInterface, objects ...any) imagedata.Context {
	return imageDataContext(ctx, provider, collectResourceImages(objects...))
}

func ImageDataContextWithResourceImages(ctx context.Context, provider imagedata.ContextInterface, resourceImages []string) imagedata.Context {
	images := make(map[string]struct{}, len(resourceImages))
	for _, image := range resourceImages {
		images[image] = struct{}{}
		if reference, err := name.ParseReference(image); err == nil {
			images[reference.Name()] = struct{}{}
		}
	}
	return imageDataContext(ctx, provider, images)
}

func imageDataContext(ctx context.Context, provider imagedata.ContextInterface, resourceImages map[string]struct{}) imagedata.Context {
	var resourceOptions sync.Map
	var images sync.Map
	if provider == nil {
		provider = LibraryContext
	}
	return imagedata.Context{ContextInterface: imageDataFunc(func(image string, options []remote.Option) (map[string]any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, resource := resourceImages[image]
		if !resource {
			if reference, err := name.ParseReference(image); err == nil {
				_, resource = resourceImages[reference.Name()]
			}
		}
		if resource {
			key := imageKey{image: image, nopts: len(options)}
			if len(options) > 0 {
				key.opts = &options[0]
			}
			cached, loaded := resourceOptions.Load(key)
			if !loaded {
				guardedOptions, err := registryclient.GuardResourceImageOptions(ctx, image, options)
				if err != nil {
					return nil, err
				}
				cached, _ = resourceOptions.LoadOrStore(key, guardedOptions)
			}
			options = cached.([]remote.Option)
		}
		if provider == nil {
			return nil, errors.New("image metadata context is not configured")
		}
		key := imageKey{image: image, nopts: len(options)}
		if len(options) > 0 {
			key.opts = &options[0]
		}
		cached, loaded := images.Load(key)
		if !loaded {
			cached, _ = images.LoadOrStore(key, &imageResult{})
		}
		result := cached.(*imageResult)
		result.once.Do(func() {
			if contextual, ok := provider.(interface {
				GetImageDataWithContext(context.Context, string, []remote.Option) (map[string]any, error)
			}); ok {
				result.data, result.err = contextual.GetImageDataWithContext(ctx, image, options)
			} else {
				result.data, result.err = provider.GetImageData(image, options)
			}
		})
		return result.data, result.err
	})}
}

func collectResourceImages(objects ...any) map[string]struct{} {
	images := make(map[string]struct{})
	for _, object := range objects {
		fields, ok := object.(map[string]any)
		if !ok {
			continue
		}
		for _, path := range [][]string{{"spec"}, {"spec", "template", "spec"}, {"spec", "jobTemplate", "spec", "template", "spec"}} {
			for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
				value, _, _ := unstructured.NestedFieldNoCopy(fields, append(path, category)...)
				containers, _ := value.([]any)
				for _, container := range containers {
					if fields, ok := container.(map[string]any); ok {
						if image, ok := fields["image"].(string); ok && image != "" {
							images[image] = struct{}{}
							if reference, err := name.ParseReference(image); err == nil {
								images[reference.Name()] = struct{}{}
							}
						}
					}
				}
			}
		}
	}
	return images
}
