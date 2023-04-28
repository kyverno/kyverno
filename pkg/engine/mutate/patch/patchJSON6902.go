package patch

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"sigs.k8s.io/yaml"
)

// ProcessPatchJSON6902 ...
func ProcessPatchJSON6902(logger logr.Logger, patchesJSON6902 []byte, resource resource) (resource, patches, error) {
	if patchedResourceRaw, err := applyPatchesWithOptions(resource, patchesJSON6902); err != nil {
		logger.Error(err, "failed to apply JSON Patch")
		return nil, nil, err
	} else if patchesBytes, err := generatePatches(resource, patchedResourceRaw); err != nil {
		return nil, nil, err
	} else {
		return patchedResourceRaw, patchesBytes, nil
	}
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

func ConvertPatchesToJSON(patchesJSON6902 string) ([]byte, error) {
	if len(patchesJSON6902) == 0 {
		return []byte(patchesJSON6902), nil
	}
	if patchesJSON6902[0] != '[' {
		// If the patch doesn't look like a JSON6902 patch, we
		// try to parse it to json.
		op, err := yaml.YAMLToJSON([]byte(patchesJSON6902))
		if err != nil {
			return nil, fmt.Errorf("failed to convert patchesJSON6902 to JSON: %v", err)
		}
		return op, nil
	}
	return []byte(patchesJSON6902), nil
}
