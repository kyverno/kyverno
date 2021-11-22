package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	openapiv2 "github.com/googleapis/gnostic/openapiv2"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	patchTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	csrtype "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	event "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

//Client enables interaction with k8 resource
type Client struct {
	client          dynamic.Interface
	log             logr.Logger
	clientConfig    *rest.Config
	kclient         kubernetes.Interface
	DiscoveryClient IDiscovery
}

//NewClient creates new instance of client
func NewClient(config *rest.Config, resync time.Duration, stopCh <-chan struct{}, log logr.Logger) (*Client, error) {

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
		log:          log.WithName("dclient"),
	}

	// Set discovery client
	discoveryClient := &ServerPreferredResources{
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

//NewDynamicSharedInformerFactory returns a new instance of DynamicSharedInformerFactory
func (c *Client) NewDynamicSharedInformerFactory(defaultResync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewDynamicSharedInformerFactory(c.client, defaultResync)
}

//GetEventsInterface provides typed interface for events
func (c *Client) GetEventsInterface() (event.EventInterface, error) {
	return c.kclient.CoreV1().Events(""), nil
}

//GetCSRInterface provides type interface for CSR
func (c *Client) GetCSRInterface() (csrtype.CertificateSigningRequestInterface, error) {
	return c.kclient.CertificatesV1beta1().CertificateSigningRequests(), nil
}

func (c *Client) getInterface(apiVersion string, kind string) dynamic.NamespaceableResourceInterface {
	return c.client.Resource(c.getGroupVersionMapper(apiVersion, kind))
}

func (c *Client) getResourceInterface(apiVersion string, kind string, namespace string) dynamic.ResourceInterface {
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
func (c *Client) getGroupVersionMapper(apiVersion string, kind string) schema.GroupVersionResource {
	if apiVersion == "" {
		gvr, _ := c.DiscoveryClient.GetGVRFromKind(kind)
		return gvr
	}

	return c.DiscoveryClient.GetGVRFromAPIVersionKind(apiVersion, kind)
}

// GetResource returns the resource in unstructured/json format
func (c *Client) GetResource(apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(apiVersion, kind, namespace).Get(context.TODO(), name, meta.GetOptions{}, subresources...)
}

//PatchResource patches the resource
func (c *Client) PatchResource(apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(apiVersion, kind, namespace).Patch(context.TODO(), name, patchTypes.JSONPatchType, patch, meta.PatchOptions{})
}

// GetDynamicInterface fetches underlying dynamic interface
func (c *Client) GetDynamicInterface() dynamic.Interface {
	return c.client
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *Client) ListResource(apiVersion string, kind string, namespace string, lselector *meta.LabelSelector) (*unstructured.UnstructuredList, error) {
	options := meta.ListOptions{}
	if lselector != nil {
		options = meta.ListOptions{LabelSelector: meta.FormatLabelSelector(lselector)}
	}

	return c.getResourceInterface(apiVersion, kind, namespace).List(context.TODO(), options)
}

// DeleteResource deletes the specified resource
func (c *Client) DeleteResource(apiVersion string, kind string, namespace string, name string, dryRun bool) error {
	options := meta.DeleteOptions{}
	if dryRun {
		options = meta.DeleteOptions{DryRun: []string{meta.DryRunAll}}
	}
	return c.getResourceInterface(apiVersion, kind, namespace).Delete(context.TODO(), name, options)

}

// CreateResource creates object for the specified resource/namespace
func (c *Client) CreateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
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
func (c *Client) UpdateResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
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
func (c *Client) UpdateStatusResource(apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
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
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		return nil
	}
	return &unstructured.Unstructured{Object: unstructuredObj}
}

//IDiscovery provides interface to mange Kind and GVR mapping
type IDiscovery interface {
	FindResource(apiVersion string, kind string) (*meta.APIResource, schema.GroupVersionResource, error)
	GetGVRFromKind(kind string) (schema.GroupVersionResource, error)
	GetGVRFromAPIVersionKind(apiVersion string, kind string) schema.GroupVersionResource
	GetServerVersion() (*version.Info, error)
	OpenAPISchema() (*openapiv2.Document, error)
	DiscoveryCache() discovery.CachedDiscoveryInterface
}

// SetDiscovery sets the discovery client implementation
func (c *Client) SetDiscovery(discoveryClient IDiscovery) {
	c.DiscoveryClient = discoveryClient
}

//ServerPreferredResources stores the cachedClient instance for discovery client
type ServerPreferredResources struct {
	cachedClient discovery.CachedDiscoveryInterface
	log          logr.Logger
}

// DiscoveryCache gets the discovery client cache
func (c ServerPreferredResources) DiscoveryCache() discovery.CachedDiscoveryInterface {
	return c.cachedClient
}

//Poll will keep invalidate the local cache
func (c ServerPreferredResources) Poll(resync time.Duration, stopCh <-chan struct{}) {
	logger := c.log.WithName("Poll")
	// start a ticker
	ticker := time.NewTicker(resync)
	defer func() { ticker.Stop() }()
	logger.V(4).Info("starting registered resources sync", "period", resync)
	for {
		select {
		case <-stopCh:
			logger.Info("stopping registered resources sync")
			return
		case <-ticker.C:
			// set cache as stale
			logger.V(6).Info("invalidating local client cache for registered resources")
			c.cachedClient.Invalidate()
		}
	}
}

// OpenAPISchema returns the API server OpenAPI schema document
func (c ServerPreferredResources) OpenAPISchema() (*openapiv2.Document, error) {
	return c.cachedClient.OpenAPISchema()
}

// GetGVRFromKind get the Group Version Resource from kind
func (c ServerPreferredResources) GetGVRFromKind(kind string) (schema.GroupVersionResource, error) {
	if kind == "" {
		return schema.GroupVersionResource{}, nil
	}

	_, gvr, err := c.FindResource("", kind)
	if err != nil {
		c.log.Info("schema not found", "kind", kind)
		return schema.GroupVersionResource{}, err
	}

	return gvr, nil
}

// GetGVRFromAPIVersionKind get the Group Version Resource from APIVersion and kind
func (c ServerPreferredResources) GetGVRFromAPIVersionKind(apiVersion string, kind string) schema.GroupVersionResource {
	_, gvr, err := c.FindResource(apiVersion, kind)
	if err != nil {
		c.log.Info("schema not found", "kind", kind, "apiVersion", apiVersion, "error : ", err)
		return schema.GroupVersionResource{}
	}

	return gvr
}

// GetServerVersion returns the server version of the cluster
func (c ServerPreferredResources) GetServerVersion() (*version.Info, error) {
	return c.cachedClient.ServerVersion()
}

// FindResource finds an API resource that matches 'kind'. If the resource is not
// found and the Cache is not fresh, the cache is invalidated and a retry is attempted
func (c ServerPreferredResources) FindResource(apiVersion string, kind string) (*meta.APIResource, schema.GroupVersionResource, error) {
	r, gvr, err := c.findResource(apiVersion, kind)
	if err == nil {
		return r, gvr, nil
	}

	if !c.cachedClient.Fresh() {
		c.cachedClient.Invalidate()
		if r, gvr, err = c.findResource(apiVersion, kind); err == nil {
			return r, gvr, nil
		}
	}

	return nil, schema.GroupVersionResource{}, err
}

func (c ServerPreferredResources) findResource(apiVersion string, kind string) (*meta.APIResource, schema.GroupVersionResource, error) {
	var serverResources []*meta.APIResourceList
	var err error
	if apiVersion == "" {
		serverResources, err = c.cachedClient.ServerPreferredResources()
	} else {
		_, serverResources, err = c.cachedClient.ServerGroupsAndResources()
	}

	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			logDiscoveryErrors(err, c)
		} else if isMetricsServerUnavailable(kind, err) {
			c.log.V(3).Info("failed to find preferred resource version", "error", err.Error())
		} else {
			c.log.Error(err, "failed to find preferred resource version")
			return nil, schema.GroupVersionResource{}, err
		}
	}

	for _, serverResource := range serverResources {
		if apiVersion != "" && serverResource.GroupVersion != apiVersion {
			continue
		}

		for _, resource := range serverResource.APIResources {
			if strings.Contains(resource.Name, "/") {
				// skip the sub-resources like deployment/status
				continue
			}

			// match kind or names (e.g. Namespace, namespaces, namespace)
			// to allow matching API paths (e.g. /api/v1/namespaces).
			if resource.Kind == kind || resource.Name == kind || resource.SingularName == kind {
				gv, err := schema.ParseGroupVersion(serverResource.GroupVersion)
				if err != nil {
					c.log.Error(err, "failed to parse groupVersion", "groupVersion", serverResource.GroupVersion)
					return nil, schema.GroupVersionResource{}, err
				}

				return &resource, gv.WithResource(resource.Name), nil
			}
		}
	}

	return nil, schema.GroupVersionResource{}, fmt.Errorf("kind '%s' not found in apiVersion '%s'", kind, apiVersion)
}

func logDiscoveryErrors(err error, c ServerPreferredResources) {
	discoveryError := err.(*discovery.ErrGroupDiscoveryFailed)
	for gv, e := range discoveryError.Groups {
		if gv.Group == "custom.metrics.k8s.io" || gv.Group == "metrics.k8s.io" || gv.Group == "external.metrics.k8s.io" {
			// These errors occur when Prometheus is installed as an external metrics server
			// See: https://github.com/kyverno/kyverno/issues/1490
			c.log.V(3).Info("failed to retrieve metrics API group", "gv", gv)
			continue
		}

		c.log.Error(e, "failed to retrieve API group", "gv", gv)
	}
}

func isMetricsServerUnavailable(kind string, err error) bool {
	// error message is defined at:
	// https://github.com/kubernetes/apimachinery/blob/2456ebdaba229616fab2161a615148884b46644b/pkg/api/errors/errors.go#L432
	return strings.HasPrefix(kind, "metrics.k8s.io/") &&
		strings.Contains(err.Error(), "the server is currently unable to handle the request")
}
