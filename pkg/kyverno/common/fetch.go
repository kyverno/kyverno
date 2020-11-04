package common

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// GetResources gets matched resources by the given policies
// the resources are fetched from
// - local paths to resources, if given
// - the k8s cluster, if given
func GetResources(policies []*v1.ClusterPolicy, resourcePaths []string, dClient *client.Client) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	var err error

	if dClient != nil {
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

		resources, err = getResourcesOfTypeFromCluster(resourceTypes, dClient)
		if err != nil {
			return nil, err
		}
	}

	for _, resourcePath := range resourcePaths {
		resourceBytes, err := getFileBytes(resourcePath)
		if err != nil {
			return nil, err
		}
		getResources, err := GetResource(resourceBytes)
		if err != nil {
			return nil, err
		}

		for _, resource := range getResources {
			resources = append(resources, resource)
		}
	}

	return resources, nil
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

func getResourcesOfTypeFromCluster(resourceTypes []string, dClient *client.Client) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured

	for _, kind := range resourceTypes {
		resourceList, err := dClient.ListResource("", kind, "", nil)
		if err != nil {
			return nil, err
		}

		version := resourceList.GetAPIVersion()
		for _, resource := range resourceList.Items {
			resource.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "",
				Version: version,
				Kind:    kind,
			})
			resources = append(resources, resource.DeepCopy())
		}
	}

	return resources, nil
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
