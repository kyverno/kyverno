package registryclient

import (
	"context"
	"slices"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func GuardResourceImageOptions(ctx context.Context, image string, options []remote.Option) ([]remote.Option, error) {
	configured, ok := globalClient.(*client)
	if !ok {
		configured = New().(*client)
	}
	if configured.configErr != nil {
		return nil, configured.configErr
	}
	reference, err := name.ParseReference(image, configured.imageNames...)
	if err != nil {
		return nil, err
	}
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := configured.policy.ResolveAndValidate(validationCtx, reference.Context().RegistryStr()); err != nil {
		return nil, err
	}
	return append(slices.Clone(options), remote.WithTransport(configured.transport), remote.WithContext(ctx)), nil
}
