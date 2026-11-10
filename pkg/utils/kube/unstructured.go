package kube

import (
	"encoding/json"

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
	// Unstructured objects already have native JSON-compatible values.
	// Re-encoding them may round int64 fields or change integral float64 types.
	if resource, ok := obj.(*unstructured.Unstructured); ok && resource != nil {
		return resource.DeepCopy(), nil
	}
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
