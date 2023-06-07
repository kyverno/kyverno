package patch

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
)

// ProcessPatchJSON6902 ...
func ProcessPatchJSON6902(logger logr.Logger, patchesJSON6902 []byte, resource resource) (resource, error) {
	patchedResourceRaw, err := applyPatchesWithOptions(resource, patchesJSON6902)
	if err != nil {
		logger.Error(err, "failed to apply JSON Patch")
		return nil, err
	}
	return patchedResourceRaw, nil
}

func applyPatchesWithOptions(resource, patch []byte) ([]byte, error) {
	patches, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return resource, fmt.Errorf("failed to decode patches: %v", err)
	}
	options := &jsonpatch.ApplyOptions{SupportNegativeIndices: true, AllowMissingPathOnRemove: true, EnsurePathExistsOnAdd: true}
	patchedResource, err := patches.ApplyWithOptions(resource, options)
	if err != nil {
		return resource, err
	}
	return patchedResource, nil
}
