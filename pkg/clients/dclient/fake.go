package dclient

import (
	"fmt"
	"strings"

	openapiv2 "github.com/google/gnostic/openapiv2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// NewFakeClient ---testing utilities
func NewFakeClient(scheme *runtime.Scheme, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) (Interface, error) {
	c := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
	// the typed and dynamic client are initialized with similar resources
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		client:  c,
		kclient: kclient,
	}, nil
}

func NewEmptyFakeClient() Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{}
	objects := []runtime.Object{}
	scheme := runtime.NewScheme()

	return &client{
		client:          fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...),
		kclient:         kubefake.NewSimpleClientset(objects...),
		discoveryClient: NewFakeDiscoveryClient(nil),
	}
}

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

func (c *fakeDiscoveryClient) DiscoveryInterface() discovery.DiscoveryInterface {
	return nil
}
