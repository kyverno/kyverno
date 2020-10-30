package common

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	client "github.com/kyverno/kyverno/pkg/dclient"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetResources gets matched resources by the given policies
// the resources are fetched from
// - local paths to resources, if given
// - the k8s cluster, if given
func GetResources(policies []*v1.ClusterPolicy, resourcePaths []string, dClient *client.Client, cluster bool, namespace string) ([]*unstructured.Unstructured, bool, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var err error
	var resourceFromCluster bool
	var resourceTypesMap = make(map[string]bool)
	var resourceTypes []string

	for _, policy := range policies {
		for _, rule := range policy.Spec.Rules {
			for _, kind := range rule.MatchResources.Kinds {
				resourceTypesMap[kind] = true
			}
		}
	}

	for kind := range resourceTypesMap {
		resourceTypes = append(resourceTypes, kind)
	}

	var resourceMap map[string]map[string]*unstructured.Unstructured
	if cluster && dClient != nil {
		resourceMap, err = getResourcesOfTypeFromCluster(resourceTypes, dClient, namespace)
		if err != nil {
			return nil, resourceFromCluster, err
		}
		if len(resourcePaths) == 0 {
			for _, rm := range resourceMap {
				for _, rr := range rm {
					resources = append(resources, rr)
				}
			}
			if resources != nil{
				resourceFromCluster = true
			}
		}
	}

	for _, resourcePath := range resourcePaths {
		resourceBytes, err := getFileBytes(resourcePath)
		if err != nil {
			// check in the cluster for the given resource name
			// what if two resources have same name ?
			//r, err := getResourceFromCluster(resourceTypes, resourcePath, dClient)
			//if err != nil {

			//}
			if cluster {
				for _, rm := range resourceMap {
					for rn, rr := range rm {
						resourceFromCluster = true
						if rn == resourcePath {
							resources = append(resources, rr)
							continue
						}
					}
				}
			} else {
				return nil, resourceFromCluster, err
			}
		}

		getResources, err := GetResource(resourceBytes)
		if err != nil {
			return nil, resourceFromCluster, err
		}
		for _, resource := range getResources {
			resources = append(resources, resource)
		}
	}
	return resources, resourceFromCluster, nil
}

func getResourceFromCluster(resourceTypes []string, resourceName string, dClient *client.Client) (*unstructured.Unstructured, error) {
	var resource *unstructured.Unstructured
	for _, kind := range resourceTypes {
		r, err := dClient.GetResource("", kind, "", resourceName, "")

		if err != nil {
			continue
		} else {
			return r, nil
		}
	}

	return resource, nil
}

// GetResource converts raw bytes to unstructured object
func GetResource(resourceBytes []byte) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var getErrString string

	files, splitDocError := utils.SplitYAMLDocuments(resourceBytes)
	if splitDocError != nil {
		return nil, splitDocError
	}

	for _, resourceYaml := range files {
		resource, err := convertResourceToUnstructured(resourceYaml)
		if err != nil {
			getErrString = getErrString + err.Error() + "\n"
		}
		resources = append(resources, resource)
	}

	if getErrString != "" {
		return nil, errors.New(getErrString)
	}

	return resources, nil
}

func getResourcesOfTypeFromCluster(resourceTypes []string, dClient *client.Client, namespace string) (map[string]map[string]*unstructured.Unstructured, error) {
	r := make(map[string]map[string]*unstructured.Unstructured)

	var resources []*unstructured.Unstructured

	for _, kind := range resourceTypes {
		r[kind] = make(map[string]*unstructured.Unstructured)
		resourceList, err := dClient.ListResource("", kind, namespace, nil)
		if err != nil {
			return nil, err
		}
		version := resourceList.GetAPIVersion()
		for _, resource := range resourceList.Items {
			r[kind][resource.GetName()] = resource.DeepCopy()
			resource.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: version,
				Kind:    kind,
			})
			resources = append(resources, resource.DeepCopy())
		}
	}
	return r, nil
}

func getFileBytes(path string) ([]byte, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return file, err
}

func convertResourceToUnstructured(resourceYaml []byte) (*unstructured.Unstructured, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	resourceObject, metaData, err := decode(resourceYaml, nil, nil)
	if err != nil {
		return nil, err
	}

	resourceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&resourceObject)
	if err != nil {
		return nil, err
	}

	resourceJSON, err := json.Marshal(resourceUnstructured)
	if err != nil {
		return nil, err
	}

	resource, err := engineutils.ConvertToUnstructured(resourceJSON)
	if err != nil {
		return nil, err
	}

	resource.SetGroupVersionKind(*metaData)

	if resource.GetNamespace() == "" {
		resource.SetNamespace("default")
	}

	return resource, nil
}
