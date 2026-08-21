package json

import (
	"strings"
)

// JoinPatches joins array of serialized JSON patches to the single JSONPatch array
// It accepts patch operations and patches (arrays of patch operations) and returns
// a single combined patch.
func JoinPatches(patches ...[]byte) []byte {
	if len(patches) == 0 {
		return nil
	}

	patchOperations := make([]string, 0, len(patches))
	for _, patch := range patches {
		str := strings.TrimSpace(string(patch))
		if len(str) == 0 {
			continue
		}

		if strings.HasPrefix(str, "[") {
			str = strings.TrimPrefix(str, "[")
			str = strings.TrimSuffix(str, "]")
			str = strings.TrimSpace(str)
		}

		// An empty array (`[]`) becomes an empty string here; skip it so it does
		// not join into the result as a stray, comma-producing operation.
		if len(str) == 0 {
			continue
		}

		patchOperations = append(patchOperations, str)
	}

	if len(patchOperations) == 0 {
		return nil
	}

	result := "[" + strings.Join(patchOperations, ", ") + "]"
	return []byte(result)
}
