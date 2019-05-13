package mutation

import (
	"encoding/json"
	"errors"

	jsonpatch "github.com/evanphx/json-patch"
	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

type PatchBytes []byte

// Test patches on given document according to given sets.
// Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
func ProcessPatches(patches []kubepolicy.Patch, resource []byte) ([]PatchBytes, error) {
	if len(resource) == 0 {
		return nil, errors.New("Source document for patching is empty")
	}

	var appliedPatches []PatchBytes
	for _, patch := range patches {
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			return nil, err
		}

		_, err = applyPatch(resource, patchRaw)
		if err != nil {
			return nil, err
		}

		appliedPatches = append(appliedPatches, patchRaw)
	}
	return appliedPatches, nil
}

// JoinPatches joins array of serialized JSON patches to the single JSONPatch array
func JoinPatches(patches []PatchBytes) PatchBytes {
	var result PatchBytes
	if len(patches) == 0 {
		return result
	}

	result = append(result, []byte("[\n")...)
	for index, patch := range patches {
		result = append(result, patch...)
		if index != (len(patches) - 1) {
			result = append(result, []byte(",\n")...)
		}
	}
	result = append(result, []byte("\n]")...)
	return result
}

// ApplyPatch applies patch for resource, returns patched resource.
func applyPatch(resource []byte, patchRaw []byte) ([]byte, error) {
	patchRaw = append([]byte{'['}, patchRaw...) // push [ forward
	patchRaw = append(patchRaw, ']')            // push ] back

	patch, err := jsonpatch.DecodePatch(patchRaw)
	if err != nil {
		return nil, err
	}

	return patch.Apply(resource)
}
