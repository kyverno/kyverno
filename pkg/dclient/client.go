package client

import (
	"errors"
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

type Client struct {
	client       dynamic.Interface
	cachedClient discovery.CachedDiscoveryInterface
	clientConfig *rest.Config
	kclient      *kubernetes.Clientset
}

func NewClient(config *rest.Config) (*Client, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		client:       client,
		clientConfig: config,
		kclient:      kclient,
		cachedClient: memory.NewMemCacheClient(kclient.Discovery()),
	}, nil
}

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

//TODO: can we use dynamic client to fetch the typed interface
// or generate a kube client value to access the interface
//GetEventsInterface provides typed interface for events
func (c *Client) GetEventsInterface() (event.EventInterface, error) {
	return c.kclient.CoreV1().Events(""), nil
}

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
	return c.getGVR(resource)
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

func (c *Client) DeleteResouce(resource string, namespace string, name string) error {
	return c.getResourceInterface(resource, namespace).Delete(name, &meta.DeleteOptions{})

}

// CreateResource creates object for the specified resource/namespace
func (c *Client) CreateResource(resource string, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(resource, namespace).Create(unstructuredObj, meta.CreateOptions{})
	}
	return nil, fmt.Errorf("Unable to create resource ")
}

// UpdateResource updates object for the specified resource/namespace
func (c *Client) UpdateResource(resource string, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(resource, namespace).Update(unstructuredObj, meta.UpdateOptions{})
	}
	return nil, fmt.Errorf("Unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *Client) UpdateStatusResource(resource string, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(resource, namespace).UpdateStatus(unstructuredObj, meta.UpdateOptions{})
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

//ConvertToRuntimeObject converts unstructed to runtime.Object runtime instance
func ConvertToRuntimeObject(obj *unstructured.Unstructured) (*runtime.Object, error) {
	scheme := runtime.NewScheme()
	gvk := obj.GroupVersionKind()
	runtimeObj, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &runtimeObj); err != nil {
		return nil, err
	}
	return &runtimeObj, nil
}

// only support 2 levels of keys
// To-Do support multiple levels of key
func keysExist(data map[string]interface{}, keys ...string) bool {
	var v interface{}
	var t map[string]interface{}
	var ok bool
	for _, key := range keys {
		ks := strings.Split(key, ".")
		if len(ks) > 2 {
			glog.Error("Only support 2 levels of keys from root. Support to be extendend in future")
			return false
		}
		if v, ok = data[ks[0]]; !ok {
			glog.Infof("key %s does not exist", key)
			return false
		}
		if len(ks) == 2 {
			if t, ok = v.(map[string]interface{}); !ok {
				glog.Error("expecting type map[string]interface{}")
			}
			return keyExist(t, ks[1])
		}
	}
	return true
}

func keyExist(data map[string]interface{}, key string) (ok bool) {
	if _, ok = data[key]; !ok {
		glog.Infof("key %s does not exist", key)
	}
	return ok
}

// support mode 'data' -> create resource
// To-Do: support 'from' -> copy/clone the resource
func (c *Client) GenerateResource(generator types.Generation, namespace string) error {
	var err error
	rGVR := c.getGVRFromKind(generator.Kind)
	resource := &unstructured.Unstructured{}

	var rdata map[string]interface{}
	// data -> create new resource
	if generator.Data != nil {
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&generator.Data)
		if err != nil {
			utilruntime.HandleError(err)
			return err
		}
		// verify if mandatory attributes have been defined
		if !keysExist(rdata, "kind", "apiVersion", "metadata.name", "metadata.namespace") {
			return errors.New("mandatory keys not defined")
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
	resource.SetNamespace(generator.Namespace)
	resource.SetResourceVersion("")

	err = c.waitUntilNamespaceIsCreated(namespace)
	if err != nil {
		glog.Errorf("Can't create a resource %s: %v", generator.Name, err)
		return nil
	}
	_, err = c.CreateResource(rGVR.Resource, generator.Namespace, resource)
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

	var lastError error = nil
	for time.Now().Sub(timeStart) < namespaceCreationMaxWaitTime {
		_, lastError = c.GetResource(Namespaces, "", name)
		if lastError == nil {
			break
		}
		time.Sleep(namespaceCreationWaitInterval)
	}
	return lastError
}

func (c *Client) getGVR(resource string) schema.GroupVersionResource {
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
	for gvr, _ := range resources {
		if gvr.Resource == resource {
			return gvr
		}
	}
	return emptyGVR
}

func (c *Client) getGVRFromKind(kind string) schema.GroupVersionResource {
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
