package data

import (
	"encoding/json"
	"errors"
	"reflect"

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

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Struct:
		mapData := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			mapData[field.Name] = v.Field(i).Interface()
		}
		return mapData, nil
	case reflect.Map:
		mapData := make(map[string]interface{})
		for _, key := range v.MapKeys() {
			keyStr, ok := key.Interface().(string)
			if !ok {
				return nil, errors.New("map key is not a string")
			}
			mapData[keyStr] = v.MapIndex(key).Interface()
		}
		return mapData, nil
	default:
		// Fallback to JSON marshaling and unmarshaling for other cases
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
}

// SliceContains checks whether values are contained in slice
func SliceContains(slice []string, values ...string) bool {
	return sets.New(slice...).HasAny(values...)
}
