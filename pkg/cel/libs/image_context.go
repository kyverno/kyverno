package libs

import (
	"context"
	"errors"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/cel/libs/imagedata"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type imageDataFunc func(string, []remote.Option) (map[string]any, error)

func (f imageDataFunc) GetImageData(image string, options []remote.Option) (map[string]any, error) {
	return f(image, options)
}

// ImageDataContext binds the image CEL receiver to one evaluation's context.
// The shared provider and compiled CEL programs remain unchanged and can be
// reused concurrently. Providers without the context extension retain their
// existing behavior, including CLI fixtures.
func ImageDataContext(ctx context.Context, provider imagedata.ContextInterface, objects ...any) imagedata.Context {
	resourceImages := collectResourceImages(objects...)
	if provider == nil {
		provider = LibraryContext
	}
	return imagedata.Context{ContextInterface: imageDataFunc(func(image string, options []remote.Option) (map[string]any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, resource := resourceImages[image]; resource {
			var err error
			options, err = registryclient.GuardResourceImageOptions(ctx, image, options)
			if err != nil {
				return nil, err
			}
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

func collectResourceImages(objects ...any) map[string]struct{} {
	images := make(map[string]struct{})
	for _, object := range objects {
		fields, ok := object.(map[string]any)
		if !ok {
			continue
		}
		for _, path := range [][]string{{"spec"}, {"spec", "template", "spec"}, {"spec", "jobTemplate", "spec", "template", "spec"}} {
			for _, category := range []string{"containers", "initContainers", "ephemeralContainers"} {
				containers, _, _ := unstructured.NestedSlice(fields, append(path, category)...)
				for _, container := range containers {
					if fields, ok := container.(map[string]any); ok {
						if image, ok := fields["image"].(string); ok && image != "" {
							images[image] = struct{}{}
						}
					}
				}
			}
		}
	}
	return images
}
