package patch

import (
	"strings"

	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"gomodules.xyz/jsonpatch/v2"
)

func ConvertPatches(in ...jsonpatch.JsonPatchOperation) [][]byte {
	var out [][]byte
	for _, patch := range in {
		if patch, err := patch.MarshalJSON(); err == nil {
			out = append(out, patch)
		}
	}
	return out
}

func generatePatches(src, dst []byte) ([]jsonpatch.JsonPatchOperation, error) {
	if pp, err := jsonpatch.CreatePatch(src, dst); err != nil {
		return nil, err
	} else {
		return filterInvalidPatches(pp), err
	}
}

// filterInvalidPatches filters out patch with the following path:
// - not */metadata/name, */metadata/namespace, */metadata/labels, */metadata/annotations
// - /status
func filterInvalidPatches(patches []jsonpatch.JsonPatchOperation) []jsonpatch.JsonPatchOperation {
	res := []jsonpatch.JsonPatchOperation{}
	for _, patch := range patches {
		if ignorePatch(patch.Path) {
			continue
		}
		res = append(res, patch)
	}
	return res
}

func ignorePatch(path string) bool {
	if wildcard.Match("/spec/triggers/*/metadata/*", path) {
		return false
	}
	if wildcard.Match("*/metadata", path) {
		return false
	}
	if strings.Contains(path, "/metadata") {
		if !strings.Contains(path, "/metadata/name") &&
			!strings.Contains(path, "/metadata/namespace") &&
			!strings.Contains(path, "/metadata/annotations") &&
			!strings.Contains(path, "/metadata/labels") &&
			!strings.Contains(path, "/metadata/ownerReferences") &&
			!strings.Contains(path, "/metadata/generateName") &&
			!strings.Contains(path, "/metadata/finalizers") {
			return true
		}
	}
	return false
}
