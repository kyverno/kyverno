package data

import (
	"encoding/json"

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
