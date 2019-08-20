package client

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

const (
	// Kind names are case sensitive
	//CSRs CertificateSigningRequest
	CSRs string = "CertificateSigningRequest"
	// Secrets Secret
	Secrets string = "Secret"
	// ConfigMaps ConfigMap
	ConfigMaps string = "ConfigMap"
	// Namespaces Namespace
	Namespaces string = "Namespace"
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
		Kclient: kclient,
	}, nil

}

// NewFakeDiscoveryClient returns a fakediscovery client
func NewFakeDiscoveryClient(registeredResouces []schema.GroupVersionResource) *fakeDiscoveryClient {
	// Load some-preregistd resources
	res := []schema.GroupVersionResource{
		schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		schema.GroupVersionResource{Version: "v1", Resource: "endpoints"},
		schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
		schema.GroupVersionResource{Version: "v1", Resource: "resourcequotas"},
		schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
		schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
	}
	registeredResouces = append(registeredResouces, res...)
	return &fakeDiscoveryClient{registeredResouces: registeredResouces}
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

func (c *fakeDiscoveryClient) GetGVRFromKind(kind string) schema.GroupVersionResource {
	resource := strings.ToLower(kind) + "s"
	return c.getGVR(resource)
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func newUnstructuredWithSpec(apiVersion, kind, namespace, name string, spec map[string]interface{}) *unstructured.Unstructured {
	u := newUnstructured(apiVersion, kind, namespace, name)
	u.Object["spec"] = spec
	return u
}

func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if s, ok := err.(stop); ok {
			return s.error
		}
		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return retry(attempts, 2*sleep, fn)
		}
		return err
	}
	return nil
}

// Custom error
type stop struct {
	error
}
