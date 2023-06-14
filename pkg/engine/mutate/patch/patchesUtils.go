package patch

import (
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/yaml"
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

func convertPatchesToJSON(patchesJSON6902 string) ([]byte, error) {
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

func generatePatches(src, dst []byte) ([]jsonpatch.JsonPatchOperation, error) {
	pp, err := jsonpatch.CreatePatch(src, dst)
	if err != nil {
		return nil, err
	}
	return pp, nil
}
