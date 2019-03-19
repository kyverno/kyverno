package webhooks

import (
	"encoding/json"
	"errors"

	jsonpatch "github.com/evanphx/json-patch"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

type PatchingSets uint8

const (
	PatchingSetsStopOnError             PatchingSets = 0
	PatchingSetsContinueOnRemoveFailure PatchingSets = 1
	PatchingSetsContinueAlways          PatchingSets = 255

	PatchingSetsDefault PatchingSets = PatchingSetsContinueOnRemoveFailure
)

type PatchBytes []byte

// Test patches on given document according to given sets.
// Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
func ProcessPatches(patches []types.PolicyPatch, originalDocument []byte, sets PatchingSets) ([]PatchBytes, error) {
	if len(originalDocument) == 0 {
		return nil, errors.New("Source document for patching is empty")
	}

	var appliedPatches []PatchBytes
	patchedDocument := originalDocument
	for _, patch := range patches {
		patchBytes, err := json.Marshal(patch)
		if err != nil && sets == PatchingSetsStopOnError {
			return nil, err
		}

		patchedDocument, err = CheckPatch(patchedDocument, patchBytes)
		if err != nil { // Ignore errors on "remove" operations
			if sets == PatchingSetsContinueOnRemoveFailure && patch.Operation == "remove" {
				continue
			} else if sets != PatchingSetsContinueAlways {
				return nil, err
			}
		} else { // In any case we should collect only valid patches
			appliedPatches = append(appliedPatches, patchBytes)
		}
	}
	return appliedPatches, nil
}

// Joins array of serialized JSON patches to the single JSONPatch array
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

// Checks patch for document, returns patched document.
// On error returns original document and error.
func CheckPatch(document []byte, patchRaw []byte) (PatchBytes, error) {
	patchRaw = append([]byte{'['}, patchRaw...) // push [ forward
	patchRaw = append(patchRaw, ']')            // push ] back
	patch, err := jsonpatch.DecodePatch(patchRaw)
	if err != nil {
		return document, err
	}

	patchedDocument, err := patch.Apply(document)
	if err != nil {
		return document, err
	}
	return patchedDocument, err
}
