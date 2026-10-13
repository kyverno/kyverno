package data

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
)

// CopyMap creates a full copy of the target map
func CopyMap(m map[string]interface{}) map[string]interface{} {
	mapCopy := make(map[string]interface{})
	for k, v := range m {
		mapCopy[k] = v
	}
	return mapCopy
}

// CopySliceOfMaps creates a full copy of the target slice
func CopySliceOfMaps(s []map[string]interface{}) []interface{} {
	if s == nil {
		return nil
	}
	sliceCopy := make([]interface{}, len(s))
	for i, v := range s {
		sliceCopy[i] = CopyMap(v)
	}
	return sliceCopy
}

func ToMap(data interface{}) (map[string]interface{}, error) {
	if m, ok := data.(map[string]interface{}); ok {
		return m, nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	mapData := make(map[string]interface{})
	err = json.Unmarshal(b, &mapData)
	if err != nil {
		return nil, err
	}
	return mapData, nil
}

// SliceContains checks whether values are contained in slice
func SliceContains(slice []string, values ...string) bool {
	return sets.New(slice...).HasAny(values...)
}

// GetStringField returns m[key] as a string, or an error if the key is
// absent or the value is not a string.
func GetStringField(m map[string]interface{}, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("field %q is missing", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q is not a string", key)
	}
	return s, nil
}
