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

func NewUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func NewUnstructuredWithSpec(apiVersion, kind, namespace, name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := NewUnstructured(apiVersion, kind, namespace, name)
	u.Object["spec"] = spec
	return u
}
