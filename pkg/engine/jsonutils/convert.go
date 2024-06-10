package jsonutils

import (
	"encoding/json"
	"fmt"
)

// DocumentToUntyped converts a typed object to JSON data
// i.e. string, []interface{}, map[string]interface{}
func DocumentToUntyped(doc interface{}) (interface{}, error) {
	switch v := doc.(type) {
	case map[string]interface{}:
		return v, nil
	case []interface{}:
		return v, nil
	case string, bool, float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, nil:
		return v, nil
	default:
		// Fallback to JSON marshalling and unmarshalling for complex types
		jsonData, err := json.Marshal(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal document: %v", err)
		}

		var untyped interface{}
		if err := json.Unmarshal(jsonData, &untyped); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
		}

		return untyped, nil
	}
}
