package json

import (
	"io"
	"strings"

	jsoniter "github.com/json-iterator/go"
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

		patchOperations = append(patchOperations, str)
	}

	if len(patchOperations) == 0 {
		return nil
	}

	result := "[" + strings.Join(patchOperations, ", ") + "]"
	return []byte(result)
}

var stdjson = jsoniter.ConfigCompatibleWithStandardLibrary

func Marshal(obj interface{}) ([]byte, error) {
	return stdjson.Marshal(obj)
}

func Unmarshal(data []byte, obj interface{}) error {
	return stdjson.Unmarshal(data, obj)
}

func MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return stdjson.MarshalIndent(v, prefix, indent)
}

func NewEncoder(w io.Writer) *jsoniter.Encoder {
	return stdjson.NewEncoder(w)
}
