package loaders

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

type imageDataLoader struct {
	ctx            context.Context //nolint:containedctx
	logger         logr.Logger
	entry          kyvernov1.ContextEntry
	enginectx      enginecontext.Interface
	jp             jmespath.Interface
	rclientFactory engineapi.RegistryClientFactory
	data           []byte
}

func NewImageDataLoader(
	ctx context.Context,
	logger logr.Logger,
	entry kyvernov1.ContextEntry,
	enginectx enginecontext.Interface,
	jp jmespath.Interface,
	rclientFactory engineapi.RegistryClientFactory,
) enginecontext.Loader {
	return &imageDataLoader{
		ctx:            ctx,
		logger:         logger,
		entry:          entry,
		enginectx:      enginectx,
		jp:             jp,
		rclientFactory: rclientFactory,
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

	client, err := idl.rclientFactory.GetClient(idl.ctx, entry.ImageRegistry.ImageRegistryCredentials)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry client %s: %v", entry.Name, err)
	}

	imageData, err := idl.fetchImageDataMap(client, refString)
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
func (idl *imageDataLoader) fetchImageDataMap(client engineapi.ImageDataClient, ref string) (interface{}, error) {
	desc, err := client.ForRef(context.Background(), ref)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image descriptor: %s, error: %v", ref, err)
	}

	var manifest interface{}
	if err := json.Unmarshal(desc.Manifest, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest for image reference: %s, error: %v", ref, err)
	}

	var configData interface{}
	if err := json.Unmarshal(desc.Config, &configData); err != nil {
		return nil, fmt.Errorf("failed to decode config for image reference: %s, error: %v", ref, err)
	}

	data := map[string]interface{}{
		"image":         desc.Image,
		"resolvedImage": desc.ResolvedImage,
		"registry":      desc.Registry,
		"repository":    desc.Repository,
		"identifier":    desc.Identifier,
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
