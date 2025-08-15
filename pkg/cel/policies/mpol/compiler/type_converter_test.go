package compiler

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type fakeOpenApiClient struct {
	paths map[string]openapi.GroupVersion
	err   error
}

func (f *fakeOpenApiClient) Paths() (map[string]openapi.GroupVersion, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.paths, nil
}

type fakeOpenApiGroupVersion struct {
	schemaBytes       []byte
	schemaErr         error
	serverRelativeURL string
}

func (f *fakeOpenApiGroupVersion) Schema(contentType string) ([]byte, error) {
	return f.schemaBytes, f.schemaErr
}
func (f *fakeOpenApiGroupVersion) ServerRelativeURL() string {
	return f.serverRelativeURL
}

func TestNewStaticTypeConverterManager(t *testing.T) {
	client := &fakeOpenApiClient{}
	manager := NewStaticTypeConverterManager(client)
	assert.NotNil(t, manager)
	assert.Implements(t, (*TypeConverterManager)(nil), manager)
}

func TestFetchPaths(t *testing.T) {
	t.Run("cached paths", func(t *testing.T) {
		m := &staticTypeConverterManager{
			fetchedPaths: map[schema.GroupVersion]openapi.GroupVersion{},
		}
		paths, err := m.fetchPaths()
		assert.NoError(t, err)
		assert.NotNil(t, paths)
	})

	t.Run("error from client", func(t *testing.T) {
		m := &staticTypeConverterManager{
			openapiClient: &fakeOpenApiClient{err: errors.New("fail")},
		}
		paths, err := m.fetchPaths()
		assert.Error(t, err)
		assert.Nil(t, paths)
	})

	t.Run("valid apis/ path", func(t *testing.T) {
		client := &fakeOpenApiClient{
			paths: map[string]openapi.GroupVersion{"apis/apps/v1": &fakeOpenApiGroupVersion{}},
		}
		m := &staticTypeConverterManager{openapiClient: client}
		paths, err := m.fetchPaths()
		assert.NoError(t, err)
		assert.Len(t, paths, 1)
	})

	t.Run("valid api/ path", func(t *testing.T) {
		client := &fakeOpenApiClient{
			paths: map[string]openapi.GroupVersion{"api/v1": &fakeOpenApiGroupVersion{}},
		}
		m := &staticTypeConverterManager{openapiClient: client}
		paths, err := m.fetchPaths()
		assert.NoError(t, err)
		assert.Len(t, paths, 1)
	})
}

func TestGetTypeConverter_AllBranches(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	t.Run("error from fetchPaths", func(t *testing.T) {
		client := &fakeOpenApiClient{err: errors.New("fail")}
		manager := NewStaticTypeConverterManager(client)
		tc := manager.GetTypeConverter(gvk)
		assert.Nil(t, tc)
	})

	t.Run("valid-gvk", func(t *testing.T) {
		manager := staticTypeConverterManager{
			fetchedPaths: map[schema.GroupVersion]openapi.GroupVersion{
				{Group: "", Version: "v1"}: &fakeOpenApiGroupVersion{},
			},
		}
		gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
		tc := manager.GetTypeConverter(gvk)
		assert.Nil(t, tc, "TypeConverter should be nil")
	})

	t.Run("no schema for gvk", func(t *testing.T) {
		client := &fakeOpenApiClient{paths: map[string]openapi.GroupVersion{"api/v1": &fakeOpenApiGroupVersion{}}}
		m := &staticTypeConverterManager{openapiClient: client}
		tc := m.GetTypeConverter(schema.GroupVersionKind{Group: "apps", Version: "v1"})
		assert.Nil(t, tc)
	})

	t.Run("existing type converter with same URL", func(t *testing.T) {
		existingTC, _ := managedfields.NewTypeConverter(map[string]*spec.Schema{}, false)
		entry := &fakeOpenApiGroupVersion{serverRelativeURL: "same"}
		m := &staticTypeConverterManager{
			fetchedPaths: map[schema.GroupVersion]openapi.GroupVersion{
				{Group: "", Version: "v1"}: entry,
			},
			typeConverterMap: map[schema.GroupVersion]typeConverterCacheEntry{
				{Group: "", Version: "v1"}: {typeConverter: existingTC, entry: &fakeOpenApiGroupVersion{serverRelativeURL: "same"}},
			},
		}
		tc := m.GetTypeConverter(gvk)
		assert.Equal(t, existingTC, tc)
	})

	t.Run("Schema() returns error", func(t *testing.T) {
		entry := &fakeOpenApiGroupVersion{schemaErr: errors.New("fail")}
		m := &staticTypeConverterManager{
			fetchedPaths: map[schema.GroupVersion]openapi.GroupVersion{{Group: "", Version: "v1"}: entry},
		}
		tc := m.GetTypeConverter(gvk)
		assert.Nil(t, tc)
	})

	t.Run("successful type converter creation", func(t *testing.T) {
		// Create schema with at least one definition
		sch := spec3.OpenAPI{
			Components: &spec3.Components{
				Schemas: map[string]*spec.Schema{"test": {}},
			},
		}
		b, _ := json.Marshal(sch)
		entry := &fakeOpenApiGroupVersion{schemaBytes: b}
		m := &staticTypeConverterManager{
			fetchedPaths:     map[schema.GroupVersion]openapi.GroupVersion{{Group: "", Version: "v1"}: entry},
			typeConverterMap: make(map[schema.GroupVersion]typeConverterCacheEntry),
		}
		tc := m.GetTypeConverter(gvk)
		assert.NotNil(t, tc)
		assert.Contains(t, m.typeConverterMap, gvk.GroupVersion())
	})
}
