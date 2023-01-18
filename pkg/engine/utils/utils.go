package utils

import (
	jsonpatch "github.com/evanphx/json-patch/v5"
	commonAnchor "github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/logging"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
)

// ApplyPatches patches given resource with given patches and returns patched document
// return original resource if any error occurs
func ApplyPatches(resource []byte, patches [][]byte) ([]byte, error) {
	if len(patches) == 0 {
		return resource, nil
	}
	joinedPatches := jsonutils.JoinPatches(patches...)
	patch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		logging.V(4).Info("failed to decode JSON patch", "patch", patch)
		return resource, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		logging.V(4).Info("failed to apply JSON patch", "patch", patch)
		return resource, err
	}

	logging.V(4).Info("applied JSON patch", "patch", patch)
	return patchedDocument, err
}

// ApplyPatchNew patches given resource with given joined patches
func ApplyPatchNew(resource, patch []byte) ([]byte, error) {
	jsonpatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return resource, err
	}

	patchedResource, err := jsonpatch.Apply(resource)
	if err != nil {
		return resource, err
	}

	return patchedResource, err
}

// GetAnchorsFromMap gets the conditional anchor map
func GetAnchorsFromMap(anchorsMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range anchorsMap {
		if commonAnchor.IsConditionAnchor(key) {
			result[key] = value
		}
	}

	return result
}
