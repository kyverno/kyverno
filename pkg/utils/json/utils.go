package json

// JoinPatches joins array of serialized JSON patches to the single JSONPatch array
func JoinPatches(patches ...[]byte) []byte {
	var result []byte
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
