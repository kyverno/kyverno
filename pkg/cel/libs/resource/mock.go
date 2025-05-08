package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ContextMock struct {
	ListResourcesFunc func(string, string, string) (*unstructured.UnstructuredList, error)
	GetResourceFunc   func(string, string, string, string) (*unstructured.Unstructured, error)
	PostResourceFunc  func(string, string, string, map[string]any) (*unstructured.Unstructured, error)
}

func (mock *ContextMock) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return mock.ListResourcesFunc(apiVersion, resource, namespace)
}

func (mock *ContextMock) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return mock.GetResourceFunc(apiVersion, resource, namespace, name)
}

func (mock *ContextMock) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return mock.PostResourceFunc(apiVersion, resource, namespace, data)
}
