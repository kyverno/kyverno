package utils

import (
	"encoding/json"
	"reflect"

	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func ConvertToNative[T any](value ref.Val) (T, error) {
	// try to convert value to native type
	response, err := value.ConvertToNative(reflect.TypeFor[T]())
	// if it failed return default value for T and error
	if err != nil {
		var t T
		return t, err
	}
	// return the converted value
	return response.(T), nil
}

func ConvertObjectToUnstructured(obj any) (*unstructured.Unstructured, error) {
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return &unstructured.Unstructured{Object: nil}, nil
	}
	ret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: ret}, nil
}

func ObjectToResolveVal(r runtime.Object) (any, error) {
	if r == nil || reflect.ValueOf(r).IsNil() {
		return nil, nil
	}
	v, err := ConvertObjectToUnstructured(r)
	if err != nil {
		return nil, err
	}
	return v.Object, nil
}

func GetValue(data any) (map[string]any, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var apiData map[string]any
	err = json.Unmarshal(raw, &apiData)
	if err != nil {
		return nil, err
	}
	return apiData, nil
}
