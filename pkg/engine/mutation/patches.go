package mutation

import (
	"encoding/json"
	"errors"
	"log"

	jsonpatch "github.com/evanphx/json-patch"
	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

type PatchBytes []byte

// ProcessPatches Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
func ProcessPatches(patches []kubepolicy.Patch, resource []byte) ([]PatchBytes, []byte, error) {
	if len(resource) == 0 {
		return nil, nil, errors.New("Source document for patching is empty")
	}

	var appliedPatches []PatchBytes
	patchedDocument := resource
	for i, patch := range patches {
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			return nil, nil, err
		}

		patchedDocument, err = applyPatch(patchedDocument, patchRaw)
		if err != nil {
			// TODO: continue on error if one of the patches fails, will add the failure event in such case
			if patch.Operation == "remove" {
				continue
			}
			log.Printf("Patch failed: patch number = %d, patch Operation = %s, err: %v", i, patch.Operation, err)
			continue
		}

		appliedPatches = append(appliedPatches, patchRaw)
	}
	return appliedPatches, patchedDocument, nil
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
		if index != len(patches)-1 {
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

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		return resource, err
	}
	return patchedDocument, err
}
