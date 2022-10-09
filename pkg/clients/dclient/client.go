package dclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyverno/kyverno/pkg/metrics"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	certsv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type Interface interface {
	// NewDynamicSharedInformerFactory returns a new instance of DynamicSharedInformerFactory
	NewDynamicSharedInformerFactory(time.Duration) dynamicinformer.DynamicSharedInformerFactory
	// GetEventsInterface provides typed interface for events
	GetEventsInterface() (corev1.EventInterface, error)
	// GetDynamicInterface fetches underlying dynamic interface
	GetDynamicInterface() dynamic.Interface
	// Discovery return the discovery client implementation
	Discovery() IDiscovery
	// SetDiscovery sets the discovery client implementation
	SetDiscovery(discoveryClient IDiscovery)
	RawAbsPath(path string) ([]byte, error)
	// GetResource returns the resource in unstructured/json format
	GetResource(apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error)
	// PatchResource patches the resource
	PatchResource(apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error)
	// ListResource returns the list of resources in unstructured/json format
	// Access items using []Items
	ListResource(apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error)
	// DeleteResource deletes the specified resource
	DeleteResource(apiVersion string, kind string, namespace string, name string, dryRun bool) error
	// CreateResource creates object for the specified resource/namespace
	CreateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// UpdateResource updates object for the specified resource/namespace
	UpdateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// UpdateStatusResource updates the resource "status" subresource
	UpdateStatusResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// RecordClientQuery publish the client query to the metric
	RecordClientQuery(clientQueryOperation metrics.ClientQueryOperation, clientType metrics.ClientType, resourceKind string, resourceNamespace string)
}

// Client enables interaction with k8 resource
type client struct {
	client          dynamic.Interface
	discoveryClient IDiscovery
	clientConfig    *rest.Config
	kclient         kubernetes.Interface
	metricsConfig   metrics.MetricsConfigManager
	restClient      rest.Interface
}

// NewClient creates new instance of client
func NewClient(ctx context.Context, config *rest.Config, kclient *kubernetes.Clientset, metricsConfig metrics.MetricsConfigManager, resync time.Duration) (Interface, error) {
	dclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	client := client{
		client:       dclient,
		clientConfig: config,
		kclient:      kclient,
		restClient:   kclient.RESTClient(),
	}

	if metricsConfig != nil {
		client.metricsConfig = metricsConfig
	}

	// Set discovery client
	discoveryClient := &serverPreferredResources{
		cachedClient: memory.NewMemCacheClient(kclient.Discovery()),
	}
	// client will invalidate registered resources cache every x seconds,
	// As there is no way to identify if the registered resource is available or not
	// we will be invalidating the local cache, so the next request get a fresh cache
	// If a resource is removed then and cache is not invalidate yet, we will not detect the removal
	// but the re-sync shall re-evaluate
	go discoveryClient.Poll(ctx, resync)
	client.SetDiscovery(discoveryClient)
	return &client, nil
}

// NewDynamicSharedInformerFactory returns a new instance of DynamicSharedInformerFactory
func (c *client) NewDynamicSharedInformerFactory(defaultResync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewDynamicSharedInformerFactory(c.client, defaultResync)
}

// GetEventsInterface provides typed interface for events
func (c *client) GetEventsInterface() (corev1.EventInterface, error) {
	return c.kclient.CoreV1().Events(""), nil
}

// GetCSRInterface provides type interface for CSR
func (c *client) GetCSRInterface() (certsv1beta1.CertificateSigningRequestInterface, error) {
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
		gvr, _ := c.discoveryClient.GetGVRFromKind(kind)
		return gvr
	}

	return c.discoveryClient.GetGVRFromAPIVersionKind(apiVersion, kind)
}

// GetResource returns the resource in unstructured/json format
func (c *client) GetResource(apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	c.RecordClientQuery(metrics.ClientGet, metrics.KubeDynamicClient, kind, namespace)
	return c.getResourceInterface(apiVersion, kind, namespace).Get(context.TODO(), name, metav1.GetOptions{}, subresources...)
}

func (c *client) RawAbsPath(path string) ([]byte, error) {
	if c.restClient == nil {
		return nil, errors.New("rest client not supported")
	}
	return c.restClient.Get().RequestURI(path).DoRaw(context.TODO())
}

// PatchResource patches the resource
func (c *client) PatchResource(apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	c.RecordClientQuery(metrics.ClientUpdate, metrics.KubeDynamicClient, kind, namespace)
	return c.getResourceInterface(apiVersion, kind, namespace).Patch(context.TODO(), name, types.JSONPatchType, patch, metav1.PatchOptions{})
}

// GetDynamicInterface fetches underlying dynamic interface
func (c *client) GetDynamicInterface() dynamic.Interface {
	return c.client
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *client) ListResource(apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	options := metav1.ListOptions{}
	if lselector != nil {
		options = metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(lselector)}
	}

	c.RecordClientQuery(metrics.ClientList, metrics.KubeDynamicClient, kind, namespace)
	return c.getResourceInterface(apiVersion, kind, namespace).List(context.TODO(), options)
}

// DeleteResource deletes the specified resource
func (c *client) DeleteResource(apiVersion string, kind string, namespace string, name string, dryRun bool) error {
	options := metav1.DeleteOptions{}
	if dryRun {
		options = metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}}
	}
	c.RecordClientQuery(metrics.ClientDelete, metrics.KubeDynamicClient, kind, namespace)
	return c.getResourceInterface(apiVersion, kind, namespace).Delete(context.TODO(), name, options)
}

// CreateResource creates object for the specified resource/namespace
func (c *client) CreateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := metav1.CreateOptions{}
	if dryRun {
		options = metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ConvertToUnstructured(obj); err == nil && unstructuredObj != nil {
		c.RecordClientQuery(metrics.ClientCreate, metrics.KubeDynamicClient, kind, namespace)
		return c.getResourceInterface(apiVersion, kind, namespace).Create(context.TODO(), unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to create resource ")
}

// UpdateResource updates object for the specified resource/namespace
func (c *client) UpdateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := metav1.UpdateOptions{}
	if dryRun {
		options = metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ConvertToUnstructured(obj); err == nil && unstructuredObj != nil {
		c.RecordClientQuery(metrics.ClientUpdate, metrics.KubeDynamicClient, kind, namespace)
		return c.getResourceInterface(apiVersion, kind, namespace).Update(context.TODO(), unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *client) UpdateStatusResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := metav1.UpdateOptions{}
	if dryRun {
		options = metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ConvertToUnstructured(obj); err == nil && unstructuredObj != nil {
		c.RecordClientQuery(metrics.ClientUpdateStatus, metrics.KubeDynamicClient, kind, namespace)
		return c.getResourceInterface(apiVersion, kind, namespace).UpdateStatus(context.TODO(), unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to update resource ")
}

// Discovery return the discovery client implementation
func (c *client) Discovery() IDiscovery {
	return c.discoveryClient
}

// SetDiscovery sets the discovery client implementation
func (c *client) SetDiscovery(discoveryClient IDiscovery) {
	c.discoveryClient = discoveryClient
}

func (c *client) RecordClientQuery(clientQueryOperation metrics.ClientQueryOperation, clientType metrics.ClientType, resourceKind string, resourceNamespace string) {
	if c.metricsConfig == nil {
		return
	}
	c.metricsConfig.RecordClientQueries(clientQueryOperation, clientType, resourceKind, resourceNamespace)
}
