package dclient

import (
	"errors"
	"fmt"
	"strings"

	openapiv2 "github.com/google/gnostic/openapiv2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		dyn:  c,
		kube: kclient,
	}, nil
}

func NewEmptyFakeClient() Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{}
	objects := []runtime.Object{}
	scheme := runtime.NewScheme()
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		dyn:   fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...),
		disco: NewFakeDiscoveryClient(nil),
		kube:  kclient,
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

func (c *fakeDiscoveryClient) getGVR(resource string) (schema.GroupVersionResource, error) {
	for _, gvr := range c.registeredResources {
		if gvr.Resource == resource {
			return gvr, nil
		}
	}
	return schema.GroupVersionResource{}, errors.New("no found")
}

func (c *fakeDiscoveryClient) GetGVKFromGVR(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (c *fakeDiscoveryClient) GetGVRFromGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	resource := strings.ToLower(gvk.Kind) + "s"
	return c.getGVR(resource)
}

func (c *fakeDiscoveryClient) FindResources(group, version, kind, subresource string) (map[TopLevelApiDescription]metav1.APIResource, error) {
	return nil, fmt.Errorf("not implemented")
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
