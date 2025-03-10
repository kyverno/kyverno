package context

import (
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MOCK FOR TESTING
type MockCtx struct {
	GetConfigMapFunc        func(string, string) (unstructured.Unstructured, error)
	GetGlobalReferenceFunc  func(string, string) (any, error)
	GetImageDataFunc        func(string) (*imagedataloader.ImageData, error)
	ParseImageReferenceFunc func(string) (imagedataloader.ImageReference, error)
	ListResourcesFunc       func(string, string, string) (*unstructured.UnstructuredList, error)
	GetResourcesFunc        func(string, string, string, string) (*unstructured.Unstructured, error)
}

func (mock *MockCtx) GetConfigMap(ns string, n string) (unstructured.Unstructured, error) {
	return mock.GetConfigMapFunc(ns, n)
}

func (mock *MockCtx) GetGlobalReference(n, p string) (any, error) {
	return mock.GetGlobalReferenceFunc(n, p)
}

func (mock *MockCtx) GetImageData(n string) (*imagedataloader.ImageData, error) {
	return mock.GetImageDataFunc(n)
}

func (mock *MockCtx) ParseImageReference(n string) (imagedataloader.ImageReference, error) {
	return mock.ParseImageReferenceFunc(n)
}

func (mock *MockCtx) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return mock.ListResourcesFunc(apiVersion, resource, namespace)
}

func (mock *MockCtx) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return mock.GetResourcesFunc(apiVersion, resource, namespace, name)
}

type mockGctxStore struct {
	data map[string]store.Entry
}

func (m *mockGctxStore) Get(name string) (store.Entry, bool) {
	entry, ok := m.data[name]
	return entry, ok
}

func (m *mockGctxStore) Set(name string, data store.Entry) {
	if m.data == nil {
		m.data = make(map[string]store.Entry)
	}
	m.data[name] = data
}

type mockEntry struct {
	data any
	err  error
}

func (m *mockEntry) Get(_ string) (any, error) {
	return m.data, m.err
}

func (m *mockEntry) Stop() {}
