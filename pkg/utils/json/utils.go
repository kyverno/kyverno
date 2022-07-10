package json

import (
	"regexp"
	"strings"
)

var space = regexp.MustCompile(`\s+`)

// JoinPatches joins array of serialized JSON patches to the single JSONPatch array
// It accepts patch operations and patches (arrays of patch operations) and returns
// a single combined patch.
func JoinPatches(patches ...[]byte) []byte {
	if len(patches) == 0 {
		return nil
	}

	var patchOperations []string
	for _, patch := range patches {
		patch = space.ReplaceAll(patch, []byte(""))
		if len(patch) == 0 {
			continue
		}

		if string(patch[0]) == "[" {
			patch = patch[1 : len(patch)-1]
		}

		patchOperations = append(patchOperations, string(patch))
	}

	if len(patchOperations) == 0 {
		return nil
	}

	result := "[" + strings.Join(patchOperations, ",\n") + "]"
	return []byte(result)
}
