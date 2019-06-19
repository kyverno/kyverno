package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	apps "k8s.io/api/apps/v1"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	csrtype "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	event "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

//Client enables interaction with k8 resource
type Client struct {
	client          dynamic.Interface
	cachedClient    discovery.CachedDiscoveryInterface
	clientConfig    *rest.Config
	kclient         kubernetes.Interface
	discoveryClient IDiscovery
}

//NewClient creates new instance of client
func NewClient(config *rest.Config) (*Client, error) {
	dclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	client := Client{
		client:       dclient,
		clientConfig: config,
		kclient:      kclient,
	}
	// Set discovery client
	discoveryClient := ServerPreferredResources{memory.NewMemCacheClient(kclient.Discovery())}
	client.SetDiscovery(discoveryClient)
	return &client, nil
}

//GetKubePolicyDeployment returns kube policy depoyment value
func (c *Client) GetKubePolicyDeployment() (*apps.Deployment, error) {
	kubePolicyDeployment, err := c.GetResource("deployments", config.KubePolicyNamespace, config.KubePolicyDeploymentName)
	if err != nil {
		return nil, err
	}
	deploy := apps.Deployment{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(kubePolicyDeployment.UnstructuredContent(), &deploy); err != nil {
		return nil, err
	}
	return &deploy, nil
}

//GetEventsInterface provides typed interface for events
//TODO: can we use dynamic client to fetch the typed interface
// or generate a kube client value to access the interface
func (c *Client) GetEventsInterface() (event.EventInterface, error) {
	return c.kclient.CoreV1().Events(""), nil
}

//GetCSRInterface provides type interface for CSR
func (c *Client) GetCSRInterface() (csrtype.CertificateSigningRequestInterface, error) {
	return c.kclient.CertificatesV1beta1().CertificateSigningRequests(), nil
}

func (c *Client) getInterface(resource string) dynamic.NamespaceableResourceInterface {
	return c.client.Resource(c.getGroupVersionMapper(resource))
}

func (c *Client) getResourceInterface(resource string, namespace string) dynamic.ResourceInterface {
	// Get the resource interface
	namespaceableInterface := c.getInterface(resource)
	// Get the namespacable interface
	var resourceInteface dynamic.ResourceInterface
	if namespace != "" {
		resourceInteface = namespaceableInterface.Namespace(namespace)
	} else {
		resourceInteface = namespaceableInterface
	}
	return resourceInteface
}

// Keep this a stateful as the resource list will be based on the kubernetes version we connect to
func (c *Client) getGroupVersionMapper(resource string) schema.GroupVersionResource {
	//TODO: add checks to see if the resource is supported
	//TODO: build the resource list dynamically( by querying the registered resources)
	//TODO: the error scenarios
	return c.discoveryClient.getGVR(resource)
}

// GetResource returns the resource in unstructured/json format
func (c *Client) GetResource(resource string, namespace string, name string) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(resource, namespace).Get(name, meta.GetOptions{})
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *Client) ListResource(resource string, namespace string) (*unstructured.UnstructuredList, error) {
	return c.getResourceInterface(resource, namespace).List(meta.ListOptions{})
}

// DeleteResouce deletes the specified resource
func (c *Client) DeleteResouce(resource string, namespace string, name string, dryRun bool) error {
	options := meta.DeleteOptions{}
	if dryRun {
		options = meta.DeleteOptions{DryRun: []string{meta.DryRunAll}}
	}
	return c.getResourceInterface(resource, namespace).Delete(name, &options)

}

// CreateResource creates object for the specified resource/namespace
func (c *Client) CreateResource(resource string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.CreateOptions{}
	if dryRun {
		options = meta.CreateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(resource, namespace).Create(unstructuredObj, options)
	}
	return nil, fmt.Errorf("Unable to create resource ")
}

// UpdateResource updates object for the specified resource/namespace
func (c *Client) UpdateResource(resource string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.UpdateOptions{}
	if dryRun {
		options = meta.UpdateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(resource, namespace).Update(unstructuredObj, options)
	}
	return nil, fmt.Errorf("Unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *Client) UpdateStatusResource(resource string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.UpdateOptions{}
	if dryRun {
		options = meta.UpdateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(resource, namespace).UpdateStatus(unstructuredObj, options)
	}
	return nil, fmt.Errorf("Unable to update resource ")
}

func convertToUnstructured(obj interface{}) *unstructured.Unstructured {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to convert : %v", err))
		return nil
	}
	return &unstructured.Unstructured{Object: unstructuredObj}
}

// GenerateResource creates resource of the specified kind(supports 'clone' & 'data')
func (c *Client) GenerateResource(generator types.Generation, namespace string) error {
	var err error
	rGVR := c.discoveryClient.getGVRFromKind(generator.Kind)
	resource := &unstructured.Unstructured{}

	var rdata map[string]interface{}
	// data -> create new resource
	if generator.Data != nil {
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&generator.Data)
		if err != nil {
			utilruntime.HandleError(err)
			return err
		}
	}
	// clone -> copy from existing resource
	if generator.Clone != nil {
		resource, err = c.GetResource(rGVR.Resource, generator.Clone.Namespace, generator.Clone.Name)
		if err != nil {
			return err
		}
		rdata = resource.UnstructuredContent()
	}

	resource.SetUnstructuredContent(rdata)
	resource.SetName(generator.Name)
	resource.SetNamespace(namespace)
	resource.SetResourceVersion("")

	err = c.waitUntilNamespaceIsCreated(namespace)
	if err != nil {
		glog.Errorf("Can't create a resource %s: %v", generator.Name, err)
		return nil
	}
	_, err = c.CreateResource(rGVR.Resource, namespace, resource, false)
	if err != nil {
		return err
	}
	return nil
}

//To-Do remove this to use unstructured type
func convertToSecret(obj *unstructured.Unstructured) (v1.Secret, error) {
	secret := v1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &secret); err != nil {
		return secret, err
	}
	return secret, nil
}

//To-Do remove this to use unstructured type
func convertToCSR(obj *unstructured.Unstructured) (*certificates.CertificateSigningRequest, error) {
	csr := certificates.CertificateSigningRequest{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &csr); err != nil {
		return nil, err
	}
	return &csr, nil
}

// Waits until namespace is created with maximum duration maxWaitTimeForNamespaceCreation
func (c *Client) waitUntilNamespaceIsCreated(name string) error {
	timeStart := time.Now()

	var lastError error
	for time.Now().Sub(timeStart) < namespaceCreationMaxWaitTime {
		_, lastError = c.GetResource(Namespaces, "", name)
		if lastError == nil {
			break
		}
		time.Sleep(namespaceCreationWaitInterval)
	}
	return lastError
}

type IDiscovery interface {
	getGVR(resource string) schema.GroupVersionResource
	getGVRFromKind(kind string) schema.GroupVersionResource
}

func (c *Client) SetDiscovery(discoveryClient IDiscovery) {
	c.discoveryClient = discoveryClient
}

type ServerPreferredResources struct {
	cachedClient discovery.CachedDiscoveryInterface
}

func (c ServerPreferredResources) getGVR(resource string) schema.GroupVersionResource {
	emptyGVR := schema.GroupVersionResource{}
	serverresources, err := c.cachedClient.ServerPreferredResources()
	if err != nil {
		utilruntime.HandleError(err)
		return emptyGVR
	}
	resources, err := discovery.GroupVersionResources(serverresources)
	if err != nil {
		utilruntime.HandleError(err)
		return emptyGVR
	}
	//TODO using cached client to support cache validation and invalidation
	// iterate over the key to compare the resource
	for gvr := range resources {
		if gvr.Resource == resource {
			return gvr
		}
	}
	return emptyGVR
}

//To-do: measure performance
//To-do: evaluate DefaultRESTMapper to fetch kind->resource mapping
func (c ServerPreferredResources) getGVRFromKind(kind string) schema.GroupVersionResource {
	emptyGVR := schema.GroupVersionResource{}
	serverresources, err := c.cachedClient.ServerPreferredResources()
	if err != nil {
		utilruntime.HandleError(err)
		return emptyGVR
	}
	for _, serverresource := range serverresources {
		for _, resource := range serverresource.APIResources {
			if resource.Kind == kind && !strings.Contains(resource.Name, "/") {
				gv, err := schema.ParseGroupVersion(serverresource.GroupVersion)
				if err != nil {
					utilruntime.HandleError(err)
					return emptyGVR
				}
				return gv.WithResource(resource.Name)
			}
		}
	}
	return emptyGVR
}
