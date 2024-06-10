package kube

import (
	"encoding/json"
	"errors"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BytesToUnstructured converts the resource to unstructured format
func BytesToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func ObjToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	if unstrObj, ok := obj.(map[string]interface{}); ok {
		return &unstructured.Unstructured{Object: unstrObj}, nil
	}

	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Struct:
		unstrObj := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			unstrObj[field.Name] = v.Field(i).Interface()
		}
		return &unstructured.Unstructured{Object: unstrObj}, nil
	case reflect.Map:
		unstrObj := make(map[string]interface{})
		for _, key := range v.MapKeys() {
			keyStr, ok := key.Interface().(string)
			if !ok {
				return nil, errors.New("map key is not a string")
			}
			unstrObj[keyStr] = v.MapIndex(key).Interface()
		}
		return &unstructured.Unstructured{Object: unstrObj}, nil
	default:
		// Fallback to JSON marshaling and unmarshaling for other cases
		raw, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		unstrObj := map[string]interface{}{}
		err = json.Unmarshal(raw, &unstrObj)
		if err != nil {
			return nil, err
		}
		return &unstructured.Unstructured{Object: unstrObj}, nil
	}
}
