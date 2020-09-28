package generate

import (
	"context"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

type E2EClient struct {
	ctx    context.Context
	Client dynamic.Interface
}

func NewE2EClient() (*E2EClient, error) {
	e2eClient := new(E2EClient)
	e2eClient.ctx = context.Background()

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	dClient, err := dynamic.NewForConfig(config)
	e2eClient.Client = dClient
	return e2eClient, err
}

// GetGVR :- gets GroupVersionResource for dynamic client
func GetGVR(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

// CleanClusterPolicies ;- Deletes all the cluster policies
func (e2e *E2EClient) CleanClusterPolicies(gvr schema.GroupVersionResource) error {
	namespace := ""
	res, err := e2e.ListNamespacedResources(gvr, namespace)
	if err != nil {
		return err
	}
	for _, r := range res.Items {
		err = e2e.DeleteNamespacedResource(gvr, namespace, r.GetName())
		if err != nil {
			return err
		}
	}
	return nil
}

// GetNamespacedResource ...
func (e2e *E2EClient) GetNamespacedResource(gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Namespace(namespace).Get(e2e.ctx, name, metav1.GetOptions{})
}

// GetClusterResource ...
func (e2e *E2EClient) GetClusteredResource(gvr schema.GroupVersionResource, name string) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Get(e2e.ctx, name, metav1.GetOptions{})
}

// GetWithRetry :- Retry Operation till the end of retry or until it is Passed, retryCount is the Wait duration after each retry,
func GetWithRetry(sleepInterval time.Duration, retryCount int, retryFunc func() error) error {
	var err error
	for i := 0; i < retryCount; i++ {
		err = retryFunc()
		if err != nil {
			time.Sleep(sleepInterval * time.Second)
			continue
		}
	}
	return err
}

// DeleteNamespacedResource ...
func (e2e *E2EClient) DeleteNamespacedResource(gvr schema.GroupVersionResource, namespace, name string) error {
	return e2e.Client.Resource(gvr).Namespace(namespace).Delete(e2e.ctx, name, metav1.DeleteOptions{})
}

// DeleteClusterResource ...
func (e2e *E2EClient) DeleteClusteredResource(gvr schema.GroupVersionResource, name string) error {
	return e2e.Client.Resource(gvr).Delete(e2e.ctx, name, metav1.DeleteOptions{})
}

// CreateNamespacedResource ...
func (e2e *E2EClient) CreateNamespacedResource(gvr schema.GroupVersionResource, namespace string, resourceData *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Namespace(namespace).Create(e2e.ctx, resourceData, metav1.CreateOptions{})
}

// CreateClusteredResource ...
func (e2e *E2EClient) CreateClusteredResource(gvr schema.GroupVersionResource, resourceData *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Create(e2e.ctx, resourceData, metav1.CreateOptions{})
}

// ListNamespacedResources ...
func (e2e *E2EClient) ListNamespacedResources(gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	return e2e.Client.Resource(gvr).Namespace(namespace).List(e2e.ctx, metav1.ListOptions{})
}

// CreateNamespacedResource creates namespaced resources like Pods, Services, Deployments etc
func (e2e *E2EClient) CreateNamespacedResourceYaml(gvr schema.GroupVersionResource, namespace string, resourceData []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := yaml.Unmarshal(resourceData, &resource)
	if err != nil {
		return nil, err
	}
	result, err := e2e.Client.Resource(gvr).Namespace(namespace).Create(e2e.ctx, &resource, metav1.CreateOptions{})
	return result, err
}

// CreateClusteredResource creates cluster resources from YAML like Namespace, ClusterRole, ClusterRoleBinding etc ...
func (e2e *E2EClient) CreateClusteredResourceYaml(gvr schema.GroupVersionResource, resourceData []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := yaml.Unmarshal(resourceData, &resource)
	if err != nil {
		return nil, err
	}
	result, err := e2e.CreateClusteredResource(gvr, &resource)
	return result, err
}
