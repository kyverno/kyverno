package resource

import (
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MOCK FOR TESTING
type MockCtx struct {
	GetGlobalReferenceFunc func(string, string) (any, error)
	GetImageDataFunc       func(string) (map[string]any, error)
	ListResourcesFunc      func(string, string, string) (*unstructured.UnstructuredList, error)
	GetResourceFunc        func(string, string, string, string) (*unstructured.Unstructured, error)
	PostResourceFunc       func(string, string, string, map[string]any) (*unstructured.Unstructured, error)
}

func (mock *MockCtx) GetGlobalReference(n, p string) (any, error) {
	return mock.GetGlobalReferenceFunc(n, p)
}

func (mock *MockCtx) GetImageData(n string) (map[string]any, error) {
	return mock.GetImageDataFunc(n)
}

func (mock *MockCtx) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return mock.ListResourcesFunc(apiVersion, resource, namespace)
}

func (mock *MockCtx) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return mock.GetResourceFunc(apiVersion, resource, namespace, name)
}

func (mock *MockCtx) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return mock.PostResourceFunc(apiVersion, resource, namespace, data)
}

type MockGctxStore struct {
	Data map[string]store.Entry
}

func (m *MockGctxStore) Get(name string) (store.Entry, bool) {
	entry, ok := m.Data[name]
	return entry, ok
}

func (m *MockGctxStore) Set(name string, data store.Entry) {
	if m.Data == nil {
		m.Data = make(map[string]store.Entry)
	}
	m.Data[name] = data
}

type MockEntry struct {
	Data any
	Err  error
}

func (m *MockEntry) Get(_ string) (any, error) {
	return m.Data, m.Err
}

func (m *MockEntry) Stop() {}
