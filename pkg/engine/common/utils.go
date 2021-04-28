package common

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
