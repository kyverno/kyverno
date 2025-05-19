package dclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	metadataclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/rest"
)

type Interface interface {
	// GetKubeClient provides typed kube client
	GetKubeClient() kubernetes.Interface
	// GetEventsInterface provides typed interface for events
	GetEventsInterface() eventsv1.EventsV1Interface
	// GetDynamicInterface fetches underlying dynamic interface
	GetDynamicInterface() dynamic.Interface
	// Discovery return the discovery client implementation
	Discovery() IDiscovery
	// SetDiscovery sets the discovery client implementation
	SetDiscovery(discoveryClient IDiscovery)
	// RawAbsPath performs a raw call to the kubernetes API
	RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error)
	// GetResource returns the resource in unstructured/json format
	GetResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error)
	// PatchResource patches the resource
	PatchResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error)
	// ListResource returns the list of resources in unstructured/json format
	// Access items using []Items
	ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error)
	// DeleteResource deletes the specified resource
	DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error
	// CreateResource creates object for the specified resource/namespace
	CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// UpdateResource updates object for the specified resource/namespace
	UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error)
	// UpdateStatusResource updates the resource "status" subresource
	UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error)
	// ApplyResource applies object for the specified resource/namespace using server-side apply
	ApplyResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error)
	// ApplyStatusResource applies the resource "status" subresource using server-side apply
	ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error)
}

// Client enables interaction with k8 resource
type client struct {
	dyn   dynamic.Interface
	disco IDiscovery
	rest  rest.Interface
	kube  kubernetes.Interface
}

// NewClient creates new instance of client
func NewClient(
	ctx context.Context,
	dyn dynamic.Interface,
	kube kubernetes.Interface,
	resync time.Duration,
	crdWatcher bool,
	metadataClient metadataclient.UpstreamInterface,
) (Interface, error) {
	disco := kube.Discovery()
	client := client{
		dyn:  dyn,
		kube: kube,
		rest: disco.RESTClient(),
	}
	// Set discovery client
	discoveryClient := &serverResources{
		cachedClient: memory.NewMemCacheClient(disco),
	}
	// client will invalidate registered resources cache every x seconds,
	// As there is no way to identify if the registered resource is available or not
	// we will be invalidating the local cache, so the next request get a fresh cache
	// If a resource is removed then and cache is not invalidate yet, we will not detect the removal
	// but the re-sync shall re-evaluate
	go discoveryClient.Poll(ctx, resync)
	// If CRD watcher is enabled, then it starts the watcher
	// This watcher will watch for CRD changes and invalidate the local cache when changes occur in customresourcedefinitions
	if crdWatcher {
		go func() {
			if err := discoveryClient.CreateCRDWatcher(ctx, metadataClient); err != nil {
				logger.Error(err, "CRD watcher failed")
			}
		}()
	}
	client.SetDiscovery(discoveryClient)
	return &client, nil
}

// NewDynamicSharedInformerFactory returns a new instance of DynamicSharedInformerFactory
func (c *client) NewDynamicSharedInformerFactory(defaultResync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewDynamicSharedInformerFactory(c.dyn, defaultResync)
}

// GetKubeClient provides typed kube client
func (c *client) GetKubeClient() kubernetes.Interface {
	return c.kube
}

// GetEventsInterface provides typed interface for events
func (c *client) GetEventsInterface() eventsv1.EventsV1Interface {
	return c.kube.EventsV1()
}

func (c *client) getInterface(apiVersion string, kind string) dynamic.NamespaceableResourceInterface {
	return c.dyn.Resource(c.getGroupVersionMapper(apiVersion, kind))
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
		if kind == "" {
			return schema.GroupVersionResource{}
		}
		apiVersion, kind = kubeutils.GetKindFromGVK(kind)
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}
	}
	gvr, err := c.disco.GetGVRFromGVK(gv.WithKind(kind))
	if err != nil {
		return schema.GroupVersionResource{}
	}
	return gvr
}

// GetResource returns the resource in unstructured/json format
func (c *client) GetResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(apiVersion, kind, namespace).Get(ctx, name, metav1.GetOptions{}, subresources...)
}

// RawAbsPath performs a raw call to the kubernetes API
func (c *client) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	if c.rest == nil {
		return nil, errors.New("rest client not supported")
	}

	switch method {
	case "GET":
		return c.rest.Get().RequestURI(path).DoRaw(ctx)
	case "POST":
		return c.rest.Post().Body(dataReader).RequestURI(path).DoRaw(ctx)

	default:
		return nil, fmt.Errorf("method not supported: %s", method)
	}
}

// PatchResource patches the resource
func (c *client) PatchResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(apiVersion, kind, namespace).Patch(ctx, name, types.JSONPatchType, patch, metav1.PatchOptions{})
}

// GetDynamicInterface fetches underlying dynamic interface
func (c *client) GetDynamicInterface() dynamic.Interface {
	return c.dyn
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *client) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	options := metav1.ListOptions{}
	if lselector != nil {
		options = metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(lselector)}
	}
	return c.getResourceInterface(apiVersion, kind, namespace).List(ctx, options)
}

// DeleteResource deletes the specified resource
func (c *client) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error {
	if dryRun {
		options = metav1.DeleteOptions{DryRun: []string{metav1.DryRunAll}}
	}
	return c.getResourceInterface(apiVersion, kind, namespace).Delete(ctx, name, options)
}

// CreateResource creates object for the specified resource/namespace
func (c *client) CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := metav1.CreateOptions{}
	if dryRun {
		options = metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ObjToUnstructured(obj); err == nil && unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).Create(ctx, unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to create resource ")
}

// UpdateResource updates object for the specified resource/namespace
func (c *client) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	options := metav1.UpdateOptions{}
	if dryRun {
		options = metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ObjToUnstructured(obj); err == nil && unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).Update(ctx, unstructuredObj, options, subresources...)
	}
	return nil, fmt.Errorf("unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *client) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := metav1.UpdateOptions{}
	if dryRun {
		options = metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ObjToUnstructured(obj); err == nil && unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).UpdateStatus(ctx, unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to update resource ")
}

// ApplyResource updates object for the specified resource/namespace
func (c *client) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	// We have a different field manager for different situations, so a generated object that then goes through admission control
	// won't have the changes wiped out by any use of server-side apply in the mutation path
	options := metav1.ApplyOptions{FieldManager: "kyverno-" + fieldManager, Force: true}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ObjToUnstructured(obj); err == nil && unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).Apply(ctx, name, unstructuredObj, options, subresources...)
	}
	return nil, fmt.Errorf("unable to apply resource ")
}

// ApplyStatusResource updates the resource "status" subresource
func (c *client) ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	// We have a different field manager for different situations, so a generated object that then goes through admission control
	// won't have the changes wiped out by any use of server-side apply in the mutation path
	options := metav1.ApplyOptions{FieldManager: "kyverno-" + fieldManager, Force: true}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	// convert typed to unstructured obj
	if unstructuredObj, err := kubeutils.ObjToUnstructured(obj); err == nil && unstructuredObj != nil {
		return c.getResourceInterface(apiVersion, kind, namespace).ApplyStatus(ctx, name, unstructuredObj, options)
	}
	return nil, fmt.Errorf("unable to apply resource ")
}

// Discovery return the discovery client implementation
func (c *client) Discovery() IDiscovery {
	return c.disco
}

// SetDiscovery sets the discovery client implementation
func (c *client) SetDiscovery(discoveryClient IDiscovery) {
	c.disco = discoveryClient
}
