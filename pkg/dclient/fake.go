package client

import (
	"fmt"
	"strings"

	openapiv2 "github.com/googleapis/gnostic/openapiv2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

// NewFakeDiscoveryClient returns a fakediscovery client
func NewFakeDiscoveryClient(registeredResources []schema.GroupVersionResource) *fakeDiscoveryClient {
	// Load some-preregistered resources
	res := []schema.GroupVersionResource{
		{Version: "v1", Resource: "configmaps"},
		{Version: "v1", Resource: "endpoints"},
		{Version: "v1", Resource: "namespaces"},
		{Version: "v1", Resource: "resourcequotas"},
		{Version: "v1", Resource: "secrets"},
		{Version: "v1", Resource: "serviceaccounts"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
	}
	registeredResources = append(registeredResources, res...)
	return &fakeDiscoveryClient{registeredResources: registeredResources}
}

type fakeDiscoveryClient struct {
	registeredResources []schema.GroupVersionResource
}

func (c *fakeDiscoveryClient) getGVR(resource string) schema.GroupVersionResource {
	for _, gvr := range c.registeredResources {
		if gvr.Resource == resource {
			return gvr
		}
	}
	return schema.GroupVersionResource{}
}

func (c *fakeDiscoveryClient) GetServerVersion() (*version.Info, error) {
	return nil, nil
}

func (c *fakeDiscoveryClient) GetGVRFromKind(kind string) (schema.GroupVersionResource, error) {
	resource := strings.ToLower(kind) + "s"
	return c.getGVR(resource), nil
}

func (c *fakeDiscoveryClient) GetGVRFromAPIVersionKind(apiVersion string, kind string) schema.GroupVersionResource {
	resource := strings.ToLower(kind) + "s"
	return c.getGVR(resource)
}

func (c *fakeDiscoveryClient) FindResource(apiVersion string, kind string) (*metav1.APIResource, schema.GroupVersionResource, error) {
	return nil, schema.GroupVersionResource{}, fmt.Errorf("not implemented")
}

func (c *fakeDiscoveryClient) OpenAPISchema() (*openapiv2.Document, error) {
	return nil, nil
}

func (c *fakeDiscoveryClient) DiscoveryCache() discovery.CachedDiscoveryInterface {
	return nil
}
