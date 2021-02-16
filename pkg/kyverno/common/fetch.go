package common

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// GetResources gets matched resources by the given policies
// the resources are fetched from
// - local paths to resources, if given
// - the k8s cluster, if given
func GetResources(policies []*v1.ClusterPolicy, resourcePaths []string, dClient *client.Client, cluster bool, namespace string, policyReport bool) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var err error
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
			return nil, err
		}
		if len(resourcePaths) == 0 {
			for _, rm := range resourceMap {
				for _, rr := range rm {
					resources = append(resources, rr)
				}
			}
		} else {
			for _, resourcePath := range resourcePaths {
				lenOfResource := len(resources)
				for _, rm := range resourceMap {
					for rn, rr := range rm {
						if rn == resourcePath {
							resources = append(resources, rr)
							continue
						}
					}
				}
				if lenOfResource >= len(resources) {
					if policyReport {
						log.Log.V(3).Info(fmt.Sprintf("%s not found in cluster", resourcePath))
					} else {
						fmt.Printf("\n----------------------------------------------------------------------\nresource %s not found in cluster\n----------------------------------------------------------------------\n", resourcePath)
					}
					return nil, errors.New(fmt.Sprintf("%s not found in cluster", resourcePath))
				}
			}
		}
	} else if len(resourcePaths) > 0 {
		for _, resourcePath := range resourcePaths {
			resourceBytes, err := getFileBytes(resourcePath)
			if err != nil {
				if policyReport {
					log.Log.V(3).Info(fmt.Sprintf("failed to load resources: %s.", resourcePath), "error", err)
				} else {
					fmt.Printf("\n----------------------------------------------------------------------\nfailed to load resources: %s. \nerror: %s\n----------------------------------------------------------------------\n", resourcePath, err)
				}
				continue
			}

			getResources, err := GetResource(resourceBytes)
			if err != nil {
				return nil, err
			}

			for _, resource := range getResources {
				resources = append(resources, resource)
			}
		}
	}
	return resources, nil
}

// GetResourcesWithTest with gets matched resources by the given policies
func GetResourcesWithTest(fs billy.Filesystem, policies []*v1.ClusterPolicy, resourcePaths []string, isGit bool, policyresoucePath string) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
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
	if len(resourcePaths) > 0 {
		for _, resourcePath := range resourcePaths {
			var resourceBytes []byte
			var err error
			if isGit {
				filep, err := fs.Open(filepath.Join(policyresoucePath, resourcePath))
				if err != nil {
					fmt.Printf("Unable to open resource file: %s. error: %s", resourcePath, err)
					continue
				}
				resourceBytes, err = ioutil.ReadAll(filep)
			} else {
				resourceBytes, err = getFileBytes(resourcePath)
			}
			if err != nil {
				fmt.Printf("\n----------------------------------------------------------------------\nfailed to load resources: %s. \nerror: %s\n----------------------------------------------------------------------\n", resourcePath, err)
				continue
			}

			getResources, err := GetResource(resourceBytes)
			if err != nil {
				return nil, err
			}

			for _, resource := range getResources {
				resources = append(resources, resource)
			}
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

func getResourcesOfTypeFromCluster(resourceTypes []string, dClient *client.Client, namespace string) (map[string]map[string]*unstructured.Unstructured, error) {
	r := make(map[string]map[string]*unstructured.Unstructured)

	var resources []*unstructured.Unstructured
	for _, kind := range resourceTypes {
		r[kind] = make(map[string]*unstructured.Unstructured)
		resourceList, err := dClient.ListResource("", kind, namespace, nil)
		if err != nil {
			// return nil, err
			continue
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

	var (
		file []byte
		err  error
	)

	if strings.Contains(path, "http") {
		resp, err := http.Get(path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, err
		}

		file, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		file, err = ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}

	return file, err
}

func convertResourceToUnstructured(resourceYaml []byte) (*unstructured.Unstructured, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, metaData, err := decode(resourceYaml, nil, nil)
	if err != nil {
		return nil, err
	}

	resourceJSON, err := yaml.YAMLToJSON(resourceYaml)
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
