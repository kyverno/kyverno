package client

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	CSR    string = "certificatesigningrequests"
	Secret string = "secrets"
)
const namespaceCreationMaxWaitTime time.Duration = 30 * time.Second
const namespaceCreationWaitInterval time.Duration = 100 * time.Millisecond

var groupVersionMapper map[string]schema.GroupVersionResource
var kubeClient *kubernetes.Clientset

func getGrpVersionMapper(kind string, clientConfig *rest.Config, refresh bool) schema.GroupVersionResource {
	// build the GVK mapper
	buildGVKMapper(clientConfig, refresh)
	// Query mapper
	if val, ok := getValue(kind); ok {
		return *val
	}
	utilruntime.HandleError(fmt.Errorf("Resouce '%s' not registered", kind))
	return schema.GroupVersionResource{}
}

func buildGVKMapper(clientConfig *rest.Config, refresh bool) {
	if groupVersionMapper == nil || refresh {
		groupVersionMapper = make(map[string]schema.GroupVersionResource)
		// refresh the mapper
		if err := refreshRegisteredResources(groupVersionMapper, clientConfig); err != nil {
			utilruntime.HandleError(err)
			return
		}
	}
}

func getValue(kind string) (*schema.GroupVersionResource, bool) {
	if groupVersionMapper == nil {
		utilruntime.HandleError(fmt.Errorf("GroupVersionKind mapper is not loaded"))
		return nil, false
	}
	if val, ok := groupVersionMapper[kind]; ok {
		return &val, true
	}
	return nil, false
}

func refreshRegisteredResources(mapper map[string]schema.GroupVersionResource, clientConfig *rest.Config) error {
	// build kubernetes client
	client, err := newKubeClient(clientConfig)
	if err != nil {
		return err
	}

	// get registered server groups and resources
	_, resourceList, err := client.Discovery().ServerGroupsAndResources()
	if err != nil {
		return err
	}
	for _, apiResource := range resourceList {
		for _, resource := range apiResource.APIResources {
			grpVersion := strings.Split(apiResource.GroupVersion, "/")
			if len(grpVersion) == 2 {
				mapper[resource.Name] = schema.GroupVersionResource{
					Group:    grpVersion[0],
					Version:  grpVersion[1],
					Resource: resource.Name,
				}
			} else {
				// resources with only versions
				mapper[resource.Name] = schema.GroupVersionResource{
					Version:  apiResource.GroupVersion,
					Resource: resource.Name,
				}
			}
		}
	}
	return nil
}

func newKubeClient(clientConfig *rest.Config) (*kubernetes.Clientset, error) {
	var err error
	if kubeClient == nil {
		kubeClient, err = kubernetes.NewForConfig(clientConfig)
		if err != nil {
			return nil, err
		}
	}
	return kubeClient, nil
}
