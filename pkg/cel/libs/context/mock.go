package context

import (
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MOCK FOR TESTING
type MockCtx struct {
	GetConfigMapFunc       func(string, string) (unstructured.Unstructured, error)
	GetGlobalReferenceFunc func(string) (any, error)
	GetImageDataFunc       func(string) (*imagedataloader.ImageData, error)
	ListResourcesFunc      func(string, string, string) (*unstructured.UnstructuredList, error)
	GetResourcesFunc       func(string, string, string, string) (*unstructured.Unstructured, error)
}

func (mock *MockCtx) GetConfigMap(ns string, n string) (unstructured.Unstructured, error) {
	return mock.GetConfigMapFunc(ns, n)
}

func (mock *MockCtx) GetGlobalReference(n string) (any, error) {
	return mock.GetGlobalReferenceFunc(n)
}

func (mock *MockCtx) GetImageData(n string) (*imagedataloader.ImageData, error) {
	return mock.GetImageDataFunc(n)
}

func (mock *MockCtx) ListResource(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return mock.ListResourcesFunc(apiVersion, resource, namespace)
}

func (mock *MockCtx) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return mock.GetResourcesFunc(apiVersion, resource, namespace, name)
}
