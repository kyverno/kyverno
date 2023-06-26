package loaders

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type imageDataLoader struct {
	ctx       context.Context //nolint:containedctx
	logger    logr.Logger
	entry     kyvernov1.ContextEntry
	enginectx enginecontext.Interface
	jp        jmespath.Interface
	rclient   registryclient.Client
	data      []byte
}

func NewImageDataLoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	jp jmespath.Interface,
	rclient registryclient.Client,
) enginecontext.Loader {
	return &imageDataLoader{
		ctx:       ctx,
		logger:    logger,
		entry:     entry,
		enginectx: enginectx,
		jp:        jp,
		rclient:   rclient,
	}
}

func (idl *imageDataLoader) LoadData() error {
	return idl.loadImageData()
}

func (cml *imageDataLoader) HasLoaded() bool {
	return cml.data != nil
}

func (idl *imageDataLoader) loadImageData() error {
	if idl.data == nil {
		imageData, err := idl.fetchImageData()
		if err != nil {
			return err
		}

		idl.data, err = json.Marshal(imageData)
		if err != nil {
			return err
		}
	}

	if err := idl.enginectx.AddContextEntry(idl.entry.Name, idl.data); err != nil {
		return fmt.Errorf("failed to add resource data to context: contextEntry: %v, error: %v", idl.entry, err)
	}

	return nil
}

func (idl *imageDataLoader) fetchImageData() (interface{}, error) {
	entry := idl.entry
	ref, err := variables.SubstituteAll(idl.logger, idl.enginectx, entry.ImageRegistry.Reference)
	if err != nil {
		return nil, fmt.Errorf("ailed to substitute variables in context entry %s %s: %v", entry.Name, entry.ImageRegistry.Reference, err)
	}

	refString, ok := ref.(string)
	if !ok {
		return nil, fmt.Errorf("invalid image reference %s, image reference must be a string", ref)
	}

	path, err := variables.SubstituteAll(idl.logger, idl.enginectx, entry.ImageRegistry.JMESPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", entry.Name, entry.ImageRegistry.JMESPath, err)
	}

	imageData, err := idl.fetchImageDataMap(idl.rclient, refString)
	if err != nil {
		return nil, err
	}

	if path != "" {
		imageData, err = applyJMESPath(idl.jp, path.(string), imageData)
		if err != nil {
			return nil, fmt.Errorf("failed to apply JMESPath (%s) results to context entry %s, error: %v", entry.ImageRegistry.JMESPath, entry.Name, err)
		}
	}

	return imageData, nil
}

// FetchImageDataMap fetches image information from the remote registry.
func (idl *imageDataLoader) fetchImageDataMap(client registryclient.Client, ref string) (interface{}, error) {
	desc, err := client.FetchImageDescriptor(context.Background(), ref)
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
	var manifest interface{}
	if err := json.Unmarshal(rawManifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest for image reference: %s, error: %v", ref, err)
	}
	rawConfig, err := image.RawConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config for image reference: %s, error: %v", ref, err)
	}
	var configData interface{}
	if err := json.Unmarshal(rawConfig, &configData); err != nil {
		return nil, fmt.Errorf("failed to decode config for image reference: %s, error: %v", ref, err)
	}

	data := map[string]interface{}{
		"image":         ref,
		"resolvedImage": fmt.Sprintf("%s@%s", desc.Ref.Context().Name(), desc.Digest.String()),
		"registry":      desc.Ref.Context().RegistryStr(),
		"repository":    desc.Ref.Context().RepositoryStr(),
		"identifier":    desc.Ref.Identifier(),
		"manifest":      manifest,
		"configData":    configData,
	}
	// we need to do the conversion from struct types to an interface type so that jmespath
	// evaluation works correctly. go-jmespath cannot handle function calls like max/sum
	// for types like integers for eg. the conversion to untyped allows the stdlib json
	// to convert all the types to types that are compatible with jmespath.
	jsonDoc, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var untyped interface{}
	err = json.Unmarshal(jsonDoc, &untyped)
	if err != nil {
		return nil, err
	}
	return untyped, nil
}
