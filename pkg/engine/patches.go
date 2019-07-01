package engine

import (
	"encoding/json"
	"errors"

	"github.com/golang/glog"

	jsonpatch "github.com/evanphx/json-patch"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
)

// PatchBytes stands for []byte
type PatchBytes []byte

// ProcessPatches Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
func ProcessPatches(rule kubepolicy.Rule, resource []byte) (allPatches []PatchBytes, errs []error) {
	if len(resource) == 0 {
		errs = append(errs, errors.New("Source document for patching is empty"))
		return nil, errs
	}
	if rule.Mutation == nil {
		errs = append(errs, errors.New("No Mutation rules defined"))
		return nil, errs
	}
	patchedDocument := resource
	for _, patch := range rule.Mutation.Patches {
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		patchedDocument, err = applyPatch(patchedDocument, patchRaw)
		// TODO: continue on error if one of the patches fails, will add the failure event in such case
		if patch.Operation == "remove" {
			glog.Info(err)
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		allPatches = append(allPatches, patchRaw)
	}
	return allPatches, errs
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

// applyPatch applies patch for resource, returns patched resource.
func applyPatch(resource []byte, patchRaw []byte) ([]byte, error) {
	patchesList := []PatchBytes{patchRaw}
	return ApplyPatches(resource, patchesList)
}

// ApplyPatches patches given resource with given patches and returns patched document
func ApplyPatches(resource []byte, patches []PatchBytes) ([]byte, error) {
	joinedPatches := JoinPatches(patches)
	patch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		return nil, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		return resource, err
	}
	return patchedDocument, err
}
