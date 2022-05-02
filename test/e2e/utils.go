package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// E2EClient ...
type E2EClient struct {
	Client  dynamic.Interface
	KClient versioned.Interface
}

type APIRequest struct {
	URL  string
	Type string
	Body io.Reader
}

// NewE2EClient returns a new instance of E2EClient
func NewE2EClient() (*E2EClient, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	e2eClient := new(E2EClient)
	if dClient, err := dynamic.NewForConfig(config); err != nil {
		return nil, err
	} else {
		e2eClient.Client = dClient
	}
	if kclient, err := versioned.NewForConfig(config); err != nil {
		return nil, err
	} else {
		e2eClient.KClient = kclient
	}
	return e2eClient, nil
}

// GetGVR :- gets GroupVersionResource for dynamic client
func GetGVR(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

func (e2e *E2EClient) ClusterPolicyReady(policyName string) bool {
	return GetWithRetry(1*time.Second, 15, func() error {
		if cpol, err := e2e.KClient.KyvernoV1().ClusterPolicies().Get(context.TODO(), policyName, metav1.GetOptions{}); err != nil {
			return err
		} else {
			if !cpol.IsReady() {
				return fmt.Errorf("cluster policy %s is not ready", policyName)
			}
			time.Sleep(2 * time.Second)
			return nil
		}
	}) == nil
}

func (e2e *E2EClient) PolicyReady(namespace string, policyName string) bool {
	return GetWithRetry(1*time.Second, 15, func() error {
		if pol, err := e2e.KClient.KyvernoV1().Policies(namespace).Get(context.TODO(), policyName, metav1.GetOptions{}); err != nil {
			return err
		} else {
			if !pol.IsReady() {
				return fmt.Errorf("cluster policy %s is not ready", policyName)
			}
			time.Sleep(2 * time.Second)
			return nil
		}
	}) != nil
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
	return e2e.Client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// GetClusteredResource ...
func (e2e *E2EClient) GetClusteredResource(gvr schema.GroupVersionResource, name string) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
}

// GetWithRetry :- Retry Operation till the end of retry or until it is Passed, retryCount is the Wait duration after each retry,
func GetWithRetry(sleepInterval time.Duration, retryCount int, retryFunc func() error) error {
	var err error
	for i := 0; i < retryCount; i++ {
		err = retryFunc()
		if err == nil {
			return nil
		}

		time.Sleep(sleepInterval)
	}

	return fmt.Errorf("operation failed, retries=%v, duration=%v: %v", retryCount, sleepInterval, err)
}

// DeleteNamespacedResource ...
func (e2e *E2EClient) DeleteNamespacedResource(gvr schema.GroupVersionResource, namespace, name string) error {
	return e2e.Client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// DeleteClusteredResource ...
func (e2e *E2EClient) DeleteClusteredResource(gvr schema.GroupVersionResource, name string) error {
	var force int64 = 0
	return e2e.Client.Resource(gvr).Delete(context.TODO(), name, metav1.DeleteOptions{GracePeriodSeconds: &force})
}

// CreateNamespacedResource ...
func (e2e *E2EClient) CreateNamespacedResource(gvr schema.GroupVersionResource, namespace string, resourceData *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Namespace(namespace).Create(context.TODO(), resourceData, metav1.CreateOptions{})
}

// CreateClusteredResource ...
func (e2e *E2EClient) CreateClusteredResource(gvr schema.GroupVersionResource, resourceData *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Create(context.TODO(), resourceData, metav1.CreateOptions{})
}

// ListNamespacedResources ...
func (e2e *E2EClient) ListNamespacedResources(gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	return e2e.Client.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
}

// CreateNamespacedResourceYaml creates namespaced resources like Pods, Services, Deployments etc
func (e2e *E2EClient) CreateNamespacedResourceYaml(gvr schema.GroupVersionResource, namespace, name string, resourceData []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := yaml.Unmarshal(resourceData, &resource)
	if err != nil {
		return nil, err
	}
	if name != "" {
		resource.SetName(name)
	}
	result, err := e2e.Client.Resource(gvr).Namespace(namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return result, err
}

// CreateClusteredResourceYaml creates cluster resources from YAML like Namespace, ClusterRole, ClusterRoleBinding etc ...
func (e2e *E2EClient) CreateClusteredResourceYaml(gvr schema.GroupVersionResource, resourceData []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := yaml.Unmarshal(resourceData, &resource)
	if err != nil {
		return nil, err
	}
	result, err := e2e.CreateClusteredResource(gvr, &resource)
	return result, err
}

// UpdateClusteredResource ...
func (e2e *E2EClient) UpdateClusteredResource(gvr schema.GroupVersionResource, resourceData *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Update(context.TODO(), resourceData, metav1.UpdateOptions{})
}

// UpdateClusteredResourceYaml creates cluster resources from YAML like Namespace, ClusterRole, ClusterRoleBinding etc ...
func (e2e *E2EClient) UpdateClusteredResourceYaml(gvr schema.GroupVersionResource, resourceData []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := yaml.Unmarshal(resourceData, &resource)
	if err != nil {
		return nil, err
	}
	result, err := e2e.UpdateClusteredResource(gvr, &resource)
	return result, err
}

// UpdateNamespacedResourceYaml creates namespaced resources like Pods, Services, Deployments etc
func (e2e *E2EClient) UpdateNamespacedResourceYaml(gvr schema.GroupVersionResource, namespace string, resourceData []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := yaml.Unmarshal(resourceData, &resource)
	if err != nil {
		return nil, err
	}
	result, err := e2e.Client.Resource(gvr).Namespace(namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
	return result, err
}

// UpdateNamespacedResource ...
func (e2e *E2EClient) UpdateNamespacedResource(gvr schema.GroupVersionResource, namespace string, resourceData *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return e2e.Client.Resource(gvr).Namespace(namespace).Update(context.TODO(), resourceData, metav1.UpdateOptions{})
}

func CallAPI(request APIRequest) (*http.Response, error) {
	var response *http.Response
	switch request.Type {
	case "GET":
		resp, err := http.Get(request.URL)
		if err != nil {
			return nil, fmt.Errorf("error occurred while calling %s: %w", request.URL, err)
		}
		response = resp
	case "POST", "PUT", "DELETE", "PATCH":
		req, err := http.NewRequest(string(request.Type), request.URL, request.Body)
		if err != nil {
			return nil, fmt.Errorf("error occurred while calling %s: %w", request.URL, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error occurred while calling %s: %w", request.URL, err)
		}
		response = resp
	default:
		return nil, fmt.Errorf("error occurred while calling %s: wrong request type found", request.URL)
	}

	return response, nil
}
