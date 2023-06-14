package adapters

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type rclientAdapter struct {
	registryclient.Client
}

func RegistryClient(client registryclient.Client) engineapi.RegistryClient {
	if client == nil {
		return nil
	}
	return &rclientAdapter{client}
}

func (a *rclientAdapter) ForRef(ctx context.Context, ref string) (*engineapi.ImageData, error) {
	desc, err := a.Client.FetchImageDescriptor(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image descriptor: %s, error: %v", ref, err)
	}
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %v", ref, err)
	}
	if err != nil {
		return nil, err
	}
	image, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve image reference: %s, error: %v", ref, err)
	}
	// We need to use the raw config and manifest to avoid dropping unknown keys
	// which are not defined in GGCR structs.
	rawManifest, err := image.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest for image reference: %s, error: %v", ref, err)
	}
	rawConfig, err := image.RawConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config for image reference: %s, error: %v", ref, err)
	}
	data := engineapi.ImageData{
		Image:         ref,
		ResolvedImage: fmt.Sprintf("%s@%s", parsedRef.Context().Name(), desc.Digest.String()),
		Registry:      parsedRef.Context().RegistryStr(),
		Repository:    parsedRef.Context().RepositoryStr(),
		Identifier:    parsedRef.Identifier(),
		Manifest:      rawManifest,
		Config:        rawConfig,
	}
	return &data, nil
}
