package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	patchTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	csrtype "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	event "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type Interface interface {
	// NewDynamicSharedInformerFactory returns a new instance of DynamicSharedInformerFactory
	NewDynamicSharedInformerFactory(time.Duration) dynamicinformer.DynamicSharedInformerFactory
	// GetEventsInterface provides typed interface for events
	GetEventsInterface() (event.EventInterface, error)
	// GetCSRInterface provides type interface for CSR
	GetCSRInterface() (csrtype.CertificateSigningRequestInterface, error)
	// GetDynamicInterface fetches underlying dynamic interface
	GetDynamicInterface() dynamic.Interface
	// Discovery return the discovery client implementation
	Discovery() IDiscovery
	// SetDiscovery sets the discovery client implementation
	SetDiscovery(discoveryClient IDiscovery)
	// GetResource returns the resource in unstructured/json format
	GetResource(apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error)
	// PatchResource patches the resource
	PatchResource(apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error)
	// ListResource returns the list of resources in unstructured/json format
	// Access items using []Items
	ListResource(apiVersion string, kind string, namespace string, lselector *meta.LabelSelector) (*unstructured.UnstructuredList, error)
	// DeleteResource deletes the specified resource
	DeleteResource(apiVersion string, kind string, namespace string, name string, dryRun bool) error
	// CreateResource creates object for the specified resource/namespace
	CreateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// UpdateResource updates object for the specified resource/namespace
	UpdateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// UpdateStatusResource updates the resource "status" subresource
	UpdateStatusResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
}

// Client enables interaction with k8 resource
type client struct {
	client          dynamic.Interface
	log             logr.Logger
	clientConfig    *rest.Config
	kclient         kubernetes.Interface
	DiscoveryClient IDiscovery
}

// NewClient creates new instance of client
func NewClient(config *rest.Config, resync time.Duration, stopCh <-chan struct{}, log logr.Logger) (Interface, error) {
	dclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	client := client{
		client:       dclient,
		clientConfig: config,
		kclient:      kclient,
		log:          log.WithName("dclient"),
	}
	// Set discovery client
	discoveryClient := &serverPreferredResources{
		cachedClient: memory.NewMemCacheClient(kclient.Discovery()),
		log:          client.log,
	}
	// client will invalidate registered resources cache every x seconds,
	// As there is no way to identify if the registered resource is available or not
	// we will be invalidating the local cache, so the next request get a fresh cache
	// If a resource is removed then and cache is not invalidate yet, we will not detect the removal
	// but the re-sync shall re-evaluate
	go discoveryClient.Poll(resync, stopCh)
	client.SetDiscovery(discoveryClient)
	return &client, nil
}

// NewDynamicSharedInformerFactory returns a new instance of DynamicSharedInformerFactory
func (c *client) NewDynamicSharedInformerFactory(defaultResync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewDynamicSharedInformerFactory(c.client, defaultResync)
}

// GetEventsInterface provides typed interface for events
func (c *client) GetEventsInterface() (event.EventInterface, error) {
	return c.kclient.CoreV1().Events(""), nil
}

// GetCSRInterface provides type interface for CSR
func (c *client) GetCSRInterface() (csrtype.CertificateSigningRequestInterface, error) {
	return c.kclient.CertificatesV1beta1().CertificateSigningRequests(), nil
}

func (c *client) getInterface(apiVersion string, kind string) dynamic.NamespaceableResourceInterface {
	return c.client.Resource(c.getGroupVersionMapper(apiVersion, kind))
}

func (c *client) getResourceInterface(apiVersion string, kind string, namespace string) dynamic.ResourceInterface {
	// Get the resource interface from kind
	namespaceableInterface := c.getInterface(apiVersion, kind)
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
func (c *client) getGroupVersionMapper(apiVersion string, kind string) schema.GroupVersionResource {
	if apiVersion == "" {
		gvr, _ := c.DiscoveryClient.GetGVRFromKind(kind)
		return gvr
	}

	return c.DiscoveryClient.GetGVRFromAPIVersionKind(apiVersion, kind)
}

// GetResource returns the resource in unstructured/json format
func (c *client) GetResource(apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(apiVersion, kind, namespace).Get(context.TODO(), name, meta.GetOptions{}, subresources...)
}

// PatchResource patches the resource
func (c *client) PatchResource(apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(apiVersion, kind, namespace).Patch(context.TODO(), name, patchTypes.JSONPatchType, patch, meta.PatchOptions{})
}

// GetDynamicInterface fetches underlying dynamic interface
func (c *client) GetDynamicInterface() dynamic.Interface {
	return c.client
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *client) ListResource(apiVersion string, kind string, namespace string, lselector *meta.LabelSelector) (*unstructured.UnstructuredList, error) {
	options := meta.ListOptions{}
	if lselector != nil {
		options = meta.ListOptions{LabelSelector: meta.FormatLabelSelector(lselector)}
	}

	return c.getResourceInterface(apiVersion, kind, namespace).List(context.TODO(), options)
}

// DeleteResource deletes the specified resource
func (c *client) DeleteResource(apiVersion string, kind string, namespace string, name string, dryRun bool) error {
	options := meta.DeleteOptions{}
	if dryRun {
		options = meta.DeleteOptions{DryRun: []string{meta.DryRunAll}}
	}
	return c.getResourceInterface(apiVersion, kind, namespace).Delete(context.TODO(), name, options)

}

// CreateResource creates object for the specified resource/namespace
func (c *client) CreateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.CreateOptions{}
	if dryRun {
		options = meta.CreateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).Create(context.TODO(), unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to create resource ")
}

// UpdateResource updates object for the specified resource/namespace
func (c *client) UpdateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.UpdateOptions{}
	if dryRun {
		options = meta.UpdateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).Update(context.TODO(), unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *client) UpdateStatusResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.UpdateOptions{}
	if dryRun {
		options = meta.UpdateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).UpdateStatus(context.TODO(), unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to update resource ")
}

func convertToUnstructured(obj interface{}) *unstructured.Unstructured {
	unstrObj := map[string]interface{}{}

	raw, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	err = json.Unmarshal(raw, &unstrObj)
	if err != nil {
		return nil
	}

	return &unstructured.Unstructured{Object: unstrObj}
}

// Discovery return the discovery client implementation
func (c *client) Discovery() IDiscovery {
	return c.DiscoveryClient
}

// SetDiscovery sets the discovery client implementation
func (c *client) SetDiscovery(discoveryClient IDiscovery) {
	c.DiscoveryClient = discoveryClient
}
