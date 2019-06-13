package client

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

const (
	//CSRs certificatesigningrequests
	CSRs string = "certificatesigningrequests"
	// Secrets secrets
	Secrets string = "secrets"
	// ConfigMaps configmaps
	ConfigMaps string = "configmaps"
	// Namespaces namespaces
	Namespaces string = "namespaces"
)
const namespaceCreationMaxWaitTime time.Duration = 30 * time.Second
const namespaceCreationWaitInterval time.Duration = 100 * time.Millisecond

//---testing utilities
func NewMockClient(scheme *runtime.Scheme, objects ...runtime.Object) (*Client, error) {
	client := fake.NewSimpleDynamicClient(scheme, objects...)
	// the typed and dynamic client are initalized with similar resources
	kclient := kubernetesfake.NewSimpleClientset(objects...)
	return &Client{
		client:  client,
		kclient: kclient,
	}, nil

}

// NewFakeDiscoveryClient returns a fakediscovery client
func NewFakeDiscoveryClient(regResources map[string]string) *fakeDiscoveryClient {
	registeredResources := make([]schema.GroupVersionResource, len(regResources))
	for groupVersion, resource := range regResources {
		gv, err := schema.ParseGroupVersion(groupVersion)
		if err != nil {
			continue
		}
		registeredResources = append(registeredResources, gv.WithResource(resource))
	}
	return &fakeDiscoveryClient{registeredResouces: registeredResources}
}

type fakeDiscoveryClient struct {
	registeredResouces []schema.GroupVersionResource
}

func (c *fakeDiscoveryClient) getGVR(resource string) schema.GroupVersionResource {
	for _, gvr := range c.registeredResouces {
		if gvr.Resource == resource {
			return gvr
		}
	}
	return schema.GroupVersionResource{}
}

func (c *fakeDiscoveryClient) getGVRFromKind(kind string) schema.GroupVersionResource {
	resource := strings.ToLower(kind) + "s"
	return c.getGVR(resource)
}
