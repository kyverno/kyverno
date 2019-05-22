package client

import (
	"fmt"
	"log"
	"os"
	"time"

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
	"k8s.io/client-go/dynamic"
	csrtype "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	event "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

type Client struct {
	logger       *log.Logger
	client       dynamic.Interface
	clientConfig *rest.Config
}

func NewClient(config *rest.Config, logger *log.Logger) (*Client, error) {
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = log.New(os.Stdout, "Client : ", log.LstdFlags)
	}

	return &Client{
		logger:       logger,
		client:       client,
		clientConfig: config,
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
	kubeClient, err := newKubeClient(c.clientConfig)
	if err != nil {
		return nil, err
	}
	return kubeClient.CoreV1().Events(""), nil
}

func (c *Client) GetCSRInterface() (csrtype.CertificateSigningRequestInterface, error) {
	kubeClient, err := newKubeClient(c.clientConfig)
	if err != nil {
		return nil, err
	}

	return kubeClient.CertificatesV1beta1().CertificateSigningRequests(), nil
}

func (c *Client) getInterface(kind string) dynamic.NamespaceableResourceInterface {
	return c.client.Resource(c.getGroupVersionMapper(kind))
}

func (c *Client) getResourceInterface(kind string, namespace string) dynamic.ResourceInterface {
	// Get the resource interface
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
	//TODO: add checks to see if the kind is supported
	//TODO: build the resource list dynamically( by querying the registered resource kinds)
	//TODO: the error scenarios
	return getGrpVersionMapper(kind, c.clientConfig, false)
}

// GetResource returns the resource in unstructured/json format
func (c *Client) GetResource(kind string, namespace string, name string) (*unstructured.Unstructured, error) {
	return c.getResourceInterface(kind, namespace).Get(name, meta.GetOptions{})
}

// ListResource returns the list of resources in unstructured/json format
// Access items using []Items
func (c *Client) ListResource(kind string, namespace string) (*unstructured.UnstructuredList, error) {
	return c.getResourceInterface(kind, namespace).List(meta.ListOptions{})
}

func (c *Client) DeleteResouce(kind string, namespace string, name string) error {
	return c.getResourceInterface(kind, namespace).Delete(name, &meta.DeleteOptions{})

}

// CreateResource creates object for the specified kind/namespace
func (c *Client) CreateResource(kind string, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(kind, namespace).Create(unstructuredObj, meta.CreateOptions{})
	}
	return nil, fmt.Errorf("Unable to create resource ")
}

// UpdateResource updates object for the specified kind/namespace
func (c *Client) UpdateResource(kind string, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(kind, namespace).Update(unstructuredObj, meta.UpdateOptions{})
	}
	return nil, fmt.Errorf("Unable to update resource ")
}

// UpdateStatusResource updates the resource "status" subresource
func (c *Client) UpdateStatusResource(kind string, namespace string, obj interface{}) (*unstructured.Unstructured, error) {
	// convert typed to unstructured obj
	if unstructuredObj := convertToUnstructured(obj); unstructuredObj != nil {
		return c.getResourceInterface(kind, namespace).UpdateStatus(unstructuredObj, meta.UpdateOptions{})
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

//TODO: make this generic for all resource type
//GenerateSecret to generate secrets

func (c *Client) GenerateSecret(generator types.Generation, namespace string) error {
	c.logger.Printf("Preparing to create secret %s/%s", namespace, generator.Name)
	secret := v1.Secret{}

	//	if generator.CopyFrom != nil {
	c.logger.Printf("Copying data from secret %s/%s", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
	// Get configMap resource
	unstrSecret, err := c.GetResource(Secret, generator.CopyFrom.Namespace, generator.CopyFrom.Name)
	if err != nil {
		return err
	}
	// typed object
	secret, err = convertToSecret(unstrSecret)
	if err != nil {
		return err
	}
	//	}

	secret.ObjectMeta = meta.ObjectMeta{
		Name:      generator.Name,
		Namespace: namespace,
	}

	// Copy data from generator to the new secret
	if generator.Data != nil {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		for k, v := range generator.Data {
			secret.Data[k] = []byte(v)
		}
	}

	go c.createSecretAfterNamespaceIsCreated(secret, namespace)
	return nil
}

//TODO: make this generic for all resource type
//GenerateConfigMap to generate configMap
func (c *Client) GenerateConfigMap(generator types.Generation, namespace string) error {
	c.logger.Printf("Preparing to create configmap %s/%s", namespace, generator.Name)
	configMap := v1.ConfigMap{}

	//	if generator.CopyFrom != nil {
	c.logger.Printf("Copying data from configmap %s/%s", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
	// Get configMap resource
	unstrConfigMap, err := c.GetResource("configmaps", generator.CopyFrom.Namespace, generator.CopyFrom.Name)
	if err != nil {
		return err
	}
	// typed object
	configMap, err = convertToConfigMap(unstrConfigMap)
	if err != nil {
		return err
	}

	//	}
	configMap.ObjectMeta = meta.ObjectMeta{
		Name:      generator.Name,
		Namespace: namespace,
	}

	// Copy data from generator to the new configmap
	if generator.Data != nil {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}

		for k, v := range generator.Data {
			configMap.Data[k] = v
		}
	}
	go c.createConfigMapAfterNamespaceIsCreated(configMap, namespace)
	return nil
}

func convertToConfigMap(obj *unstructured.Unstructured) (v1.ConfigMap, error) {
	configMap := v1.ConfigMap{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &configMap); err != nil {
		return configMap, err
	}
	return configMap, nil
}

func convertToSecret(obj *unstructured.Unstructured) (v1.Secret, error) {
	secret := v1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &secret); err != nil {
		return secret, err
	}
	return secret, nil
}

func convertToCSR(obj *unstructured.Unstructured) (*certificates.CertificateSigningRequest, error) {
	csr := certificates.CertificateSigningRequest{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &csr); err != nil {
		return nil, err
	}
	return &csr, nil
}

func (c *Client) createConfigMapAfterNamespaceIsCreated(configMap v1.ConfigMap, namespace string) {
	err := c.waitUntilNamespaceIsCreated(namespace)
	if err == nil {
		_, err = c.CreateResource("configmaps", namespace, configMap)
	}
	if err != nil {
		c.logger.Printf("Can't create a configmap: %s", err)
	}
}

func (c *Client) createSecretAfterNamespaceIsCreated(secret v1.Secret, namespace string) {
	err := c.waitUntilNamespaceIsCreated(namespace)
	if err == nil {
		_, err = c.CreateResource(Secret, namespace, secret)
	}
	if err != nil {
		c.logger.Printf("Can't create a secret: %s", err)
	}
}

// Waits until namespace is created with maximum duration maxWaitTimeForNamespaceCreation
func (c *Client) waitUntilNamespaceIsCreated(name string) error {
	timeStart := time.Now()

	var lastError error = nil
	for time.Now().Sub(timeStart) < namespaceCreationMaxWaitTime {
		_, lastError = c.GetResource("namespaces", "", name)
		if lastError == nil {
			break
		}
		time.Sleep(namespaceCreationWaitInterval)
	}
	return lastError
}
