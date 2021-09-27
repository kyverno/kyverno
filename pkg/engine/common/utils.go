package common

import "encoding/json"

// CopyMap creates a full copy of the target map
func CopyMap(m map[string]interface{}) map[string]interface{} {
	mapCopy := make(map[string]interface{})
	for k, v := range m {
		mapCopy[k] = v
	}

	return mapCopy
}

// CopySlice creates a full copy of the target slice
func CopySlice(s []interface{}) []interface{} {
	sliceCopy := make([]interface{}, len(s))
	copy(sliceCopy, s)

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