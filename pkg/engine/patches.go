package engine

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/result"
)

// PatchBytes stands for []byte
type PatchBytes []byte

// ProcessPatches Returns array from separate patches that can be applied to the document
// Returns error ONLY in case when creation of resource should be denied.
func ProcessPatches(rule kubepolicy.Rule, resource []byte) ([]PatchBytes, result.RuleApplicationResult) {
	res := result.NewRuleApplicationResult(rule.Name)
	if rule.Mutation == nil || len(rule.Mutation.Patches) == 0 {
		return nil, res
	}

	if len(resource) == 0 {
		res.AddMessagef("Source document for patching is empty")
		res.Reason = result.Failed
		return nil, res
	}

	var allPatches []PatchBytes
	patchedDocument := resource
	for i, patch := range rule.Mutation.Patches {
		patchRaw, err := json.Marshal(patch)
		if err != nil {

		}

		patchedDocument, err = applyPatch(patchedDocument, patchRaw)
		if err != nil {
			// TODO: continue on error if one of the patches fails, will add the failure event in such case
			if patch.Operation == "remove" {
				continue
			}
			message := fmt.Sprintf("Patch failed: patch number = %d, patch Operation = %s, err: %v", i, patch.Operation, err)
			res.Messages = append(res.Messages, message)
			continue
		}

		allPatches = append(allPatches, patchRaw)
	}
	return allPatches, res
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
