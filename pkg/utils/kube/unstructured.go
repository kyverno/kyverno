package kube

import (
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
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
	raw, err := jsonutils.Marshal(obj)
	if err != nil {
		return nil, err
	}
	unstrObj := map[string]interface{}{}
	err = jsonutils.Unmarshal(raw, &unstrObj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstrObj}, nil
}
