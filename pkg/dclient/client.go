package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	apps "k8s.io/api/apps/v1"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	helperv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	patchTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
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
	DiscoveryClient IDiscovery
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
	//

	discoveryClient := ServerPreferredResources{memory.NewMemCacheClient(kclient.Discovery())}
	client.SetDiscovery(discoveryClient)
	return &client, nil
}

//GetKubePolicyDeployment returns kube policy depoyment value
func (c *Client) GetKubePolicyDeployment() (*apps.Deployment, error) {
	kubePolicyDeployment, err := c.GetResource("Deployment", config.KubePolicyNamespace, config.KubePolicyDeploymentName)
	if err != nil {
		return nil, err
	}
	deploy := apps.Deployment{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(kubePolicyDeployment.UnstructuredContent(), &deploy); err != nil {
		return nil, err
	}
	return &deploy, nil
}

func (c *Client) GetAppsV1Interface() appsv1.AppsV1Interface {
	return c.kclient.AppsV1()
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

func (c *Client) getResourceInterface(kind string, namespace string) dynamic.ResourceInterface {
	// Get the resource interface from kind
	namespaceableInterface := c.getInterface(kind)
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
func (c *Client) getGroupVersionMapper(kind string) schema.GroupVersionResource {
	return c.DiscoveryClient.GetGVRFromKind(kind)
}

// GetResource returns the resource in unstructured/json format
func (c *Client) GetResource(kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(kind, namespace).Get(name, meta.GetOptions{}, subresources...)
}

//Patch
func (c *Client) PatchResource(kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(kind, namespace).Patch(name, patchTypes.JSONPatchType, patch, meta.PatchOptions{})
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *Client) ListResource(kind string, namespace string, lselector *meta.LabelSelector) (*unstructured.UnstructuredList, error) {
	options := meta.ListOptions{}
	if lselector != nil {
		options = meta.ListOptions{LabelSelector: helperv1.FormatLabelSelector(lselector)}
	}
	return c.getResourceInterface(kind, namespace).List(options)
}

// DeleteResouce deletes the specified resource
func (c *Client) DeleteResource(kind string, namespace string, name string, dryRun bool) error {
	options := meta.DeleteOptions{}
	if dryRun {
		options = meta.DeleteOptions{DryRun: []string{meta.DryRunAll}}
	}
	return c.getResourceInterface(kind, namespace).Delete(name, &options)

}

// CreateResource creates object for the specified resource/namespace
func (c *Client) CreateResource(kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.CreateOptions{}
	if dryRun {
		options = meta.CreateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(kind, namespace).Create(unstructuredObj, options)
	}
	return nil, fmt.Errorf("Unable to create resource ")
}

// UpdateResource updates object for the specified resource/namespace
func (c *Client) UpdateResource(kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.UpdateOptions{}
	if dryRun {
		options = meta.UpdateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(kind, namespace).Update(unstructuredObj, options)
	}
	return nil, fmt.Errorf("Unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *Client) UpdateStatusResource(kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	options := meta.UpdateOptions{}
	if dryRun {
		options = meta.UpdateOptions{DryRun: []string{meta.DryRunAll}}
	}
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(kind, namespace).UpdateStatus(unstructuredObj, options)
	}
	return nil, fmt.Errorf("Unable to update resource ")
}

func convertToUnstructured(obj interface{}) *unstructured.Unstructured {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		glog.Errorf("Unable to convert : %v", err)
		return nil
	}
	return &unstructured.Unstructured{Object: unstructuredObj}
}

// GenerateResource creates resource of the specified kind(supports 'clone' & 'data')
func (c *Client) GenerateResource(generator kyverno.Generation, namespace string, processExistingResources bool) error {
	var err error
	resource := &unstructured.Unstructured{}

	var rdata map[string]interface{}
	// data -> create new resource
	if generator.Data != nil {
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&generator.Data)
		if err != nil {
			glog.Error(err)
			return err
		}
	}
	// clone -> copy from existing resource
	if generator.Clone != (kyverno.CloneFrom{}) {
		resource, err = c.GetResource(generator.Kind, generator.Clone.Namespace, generator.Clone.Name)
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
	_, err = c.CreateResource(generator.Kind, namespace, resource, false)
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
	GetGVRFromKind(kind string) schema.GroupVersionResource
}

func (c *Client) SetDiscovery(discoveryClient IDiscovery) {
	c.DiscoveryClient = discoveryClient
}

type ServerPreferredResources struct {
	cachedClient discovery.CachedDiscoveryInterface
}

//GetGVRFromKind get the Group Version Resource from kind
// if kind is not found in first attempt we invalidate the cache,
// the retry will then fetch the new registered resources and check again
// if not found after 2 attempts, we declare kind is not found
// kind is Case sensitive
func (c ServerPreferredResources) GetGVRFromKind(kind string) schema.GroupVersionResource {
	var gvr schema.GroupVersionResource
	var err error
	gvr, err = loadServerResources(kind, c.cachedClient)
	if err != nil && !c.cachedClient.Fresh() {

		// invalidate cahce & re-try once more
		c.cachedClient.Invalidate()
		gvr, err = loadServerResources(kind, c.cachedClient)
		if err == nil {
			return gvr
		}
	}
	return gvr
}

func loadServerResources(k string, cdi discovery.CachedDiscoveryInterface) (schema.GroupVersionResource, error) {
	serverresources, err := cdi.ServerPreferredResources()
	emptyGVR := schema.GroupVersionResource{}
	if err != nil {
		glog.Error(err)
		return emptyGVR, err
	}
	for _, serverresource := range serverresources {
		for _, resource := range serverresource.APIResources {
			// skip the resource names with "/", to avoid comparison with subresources

			if resource.Kind == k && !strings.Contains(resource.Name, "/") {
				gv, err := schema.ParseGroupVersion(serverresource.GroupVersion)
				if err != nil {
					glog.Error(err)
					return emptyGVR, err
				}
				return gv.WithResource(resource.Name), nil
			}
		}
	}
	return emptyGVR, fmt.Errorf("kind '%s' not found", k)
}
