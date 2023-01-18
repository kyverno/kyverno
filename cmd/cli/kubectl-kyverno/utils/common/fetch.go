package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// GetResources gets matched resources by the given policies
// the resources are fetched from
// - local paths to resources, if given
// - the k8s cluster, if given
func GetResources(
	policies []kyvernov1.PolicyInterface, resourcePaths []string, dClient dclient.Interface, cluster bool,
	namespace string, policyReport bool,
) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var err error

	if cluster && dClient != nil {
		resourceTypesMap := make(map[schema.GroupVersionKind]bool)
		var resourceTypes []schema.GroupVersionKind
		var subresourceMap map[schema.GroupVersionKind]Subresource

		for _, policy := range policies {
			for _, rule := range autogen.ComputeRules(policy) {
				var resourceTypesInRule map[schema.GroupVersionKind]bool
				resourceTypesInRule, subresourceMap = GetKindsFromRule(rule, dClient)
				for resourceKind := range resourceTypesInRule {
					resourceTypesMap[resourceKind] = true
				}
			}
		}

		for kind := range resourceTypesMap {
			resourceTypes = append(resourceTypes, kind)
		}

		resources, err = whenClusterIsTrue(resourceTypes, subresourceMap, dClient, namespace, resourcePaths, policyReport)
		if err != nil {
			return resources, err
		}
	} else if len(resourcePaths) > 0 {
		resources, err = whenClusterIsFalse(resourcePaths, policyReport)
		if err != nil {
			return resources, err
		}
	}
	return resources, err
}

func whenClusterIsTrue(resourceTypes []schema.GroupVersionKind, subresourceMap map[schema.GroupVersionKind]Subresource, dClient dclient.Interface, namespace string, resourcePaths []string, policyReport bool) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	resourceMap, err := getResourcesOfTypeFromCluster(resourceTypes, subresourceMap, dClient, namespace)
	if err != nil {
		return nil, err
	}

	if len(resourcePaths) == 0 {
		for _, rr := range resourceMap {
			resources = append(resources, rr)
		}
	} else {
		for _, resourcePath := range resourcePaths {
			lenOfResource := len(resources)
			for rn, rr := range resourceMap {
				s := strings.Split(rn, "-")
				if s[2] == resourcePath {
					resources = append(resources, rr)
				}
			}

			if lenOfResource >= len(resources) {
				if policyReport {
					log.Log.V(3).Info(fmt.Sprintf("%s not found in cluster", resourcePath))
				} else {
					fmt.Printf("\n----------------------------------------------------------------------\nresource %s not found in cluster\n----------------------------------------------------------------------\n", resourcePath)
				}
				return nil, fmt.Errorf("%s not found in cluster", resourcePath)
			}
		}
	}
	return resources, nil
}

func whenClusterIsFalse(resourcePaths []string, policyReport bool) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
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

		resources = append(resources, getResources...)
	}
	return resources, nil
}

// GetResourcesWithTest with gets matched resources by the given policies
func GetResourcesWithTest(fs billy.Filesystem, policies []kyvernov1.PolicyInterface, resourcePaths []string, isGit bool, policyResourcePath string) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	resourceTypesMap := make(map[string]bool)
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			for _, kind := range rule.MatchResources.Kinds {
				resourceTypesMap[kind] = true
			}
		}
	}
	if len(resourcePaths) > 0 {
		for _, resourcePath := range resourcePaths {
			var resourceBytes []byte
			var err error
			if isGit {
				filep, err := fs.Open(filepath.Join(policyResourcePath, resourcePath))
				if err != nil {
					fmt.Printf("Unable to open resource file: %s. error: %s", resourcePath, err)
					continue
				}
				resourceBytes, _ = io.ReadAll(filep)
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

			resources = append(resources, getResources...)
		}
	}
	return resources, nil
}

// GetResource converts raw bytes to unstructured object
func GetResource(resourceBytes []byte) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var getErrString string

	files, splitDocError := yamlutils.SplitDocuments(resourceBytes)
	if splitDocError != nil {
		return nil, splitDocError
	}

	for _, resourceYaml := range files {
		resource, err := convertResourceToUnstructured(resourceYaml)
		if err != nil {
			if strings.Contains(err.Error(), "Object 'Kind' is missing") {
				log.Log.V(3).Info("skipping resource as kind not found")
				continue
			}
			getErrString = getErrString + err.Error() + "\n"
		}
		resources = append(resources, resource)
	}

	if getErrString != "" {
		return nil, errors.New(getErrString)
	}

	return resources, nil
}

func getResourcesOfTypeFromCluster(resourceTypes []schema.GroupVersionKind, subresourceMap map[schema.GroupVersionKind]Subresource, dClient dclient.Interface, namespace string) (map[string]*unstructured.Unstructured, error) {
	r := make(map[string]*unstructured.Unstructured)

	for _, kind := range resourceTypes {
		resourceList, err := dClient.ListResource(context.TODO(), kind.GroupVersion().String(), kind.Kind, namespace, nil)
		if err != nil {
			continue
		}

		gvk := resourceList.GroupVersionKind()
		for _, resource := range resourceList.Items {
			key := kind.Kind + "-" + resource.GetNamespace() + "-" + resource.GetName()
			resource.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    kind.Kind,
			})
			r[key] = resource.DeepCopy()
		}
	}

	for _, subresource := range subresourceMap {
		parentGV := schema.GroupVersion{Group: subresource.ParentResource.Group, Version: subresource.ParentResource.Version}
		resourceList, err := dClient.ListResource(context.TODO(), parentGV.String(), subresource.ParentResource.Kind, namespace, nil)
		if err != nil {
			continue
		}

		parentResourceNames := make([]string, 0)
		for _, resource := range resourceList.Items {
			parentResourceNames = append(parentResourceNames, resource.GetName())
		}

		for _, parentResourceName := range parentResourceNames {
			subresourceName := strings.Split(subresource.APIResource.Name, "/")[1]
			resource, err := dClient.GetResource(context.TODO(), parentGV.String(), subresource.ParentResource.Kind, namespace, parentResourceName, subresourceName)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
				continue
			}
			key := subresource.APIResource.Kind + "-" + resource.GetNamespace() + "-" + resource.GetName()
			resource.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   subresource.APIResource.Group,
				Version: subresource.APIResource.Version,
				Kind:    subresource.APIResource.Kind,
			})
			r[key] = resource.DeepCopy()
		}
	}
	return r, nil
}

func getFileBytes(path string) ([]byte, error) {
	var (
		file []byte
		err  error
	)

	if IsHTTPRegex.MatchString(path) {
		// We accept here that a random URL might be called based on user provided input.
		req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, err
		}

		file, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		path = filepath.Clean(path)
		// We accept the risk of including a user provided file here.
		file, err = os.ReadFile(path) // #nosec G304
		if err != nil {
			return nil, err
		}
	}

	return file, err
}

func convertResourceToUnstructured(resourceYaml []byte) (*unstructured.Unstructured, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, metaData, decodeErr := decode(resourceYaml, nil, nil)

	if decodeErr != nil {
		if !strings.Contains(decodeErr.Error(), "no kind") {
			return nil, decodeErr
		}
	}

	resourceJSON, err := yaml.YAMLToJSON(resourceYaml)
	if err != nil {
		return nil, err
	}

	resource, err := kubeutils.BytesToUnstructured(resourceJSON)
	if err != nil {
		return nil, err
	}

	if decodeErr == nil {
		resource.SetGroupVersionKind(*metaData)
	}

	if resource.GetNamespace() == "" {
		resource.SetNamespace("default")
	}
	return resource, nil
}

// GetPatchedAndGeneratedResource converts raw bytes to unstructured object
func GetPatchedAndGeneratedResource(resourceBytes []byte) (unstructured.Unstructured, error) {
	getResource, err := GetResource(resourceBytes)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	resource := *getResource[0]
	return resource, nil
}

// GetKindsFromRule will return the kinds from policy match block
func GetKindsFromRule(rule kyvernov1.Rule, client dclient.Interface) (map[schema.GroupVersionKind]bool, map[schema.GroupVersionKind]Subresource) {
	resourceTypesMap := make(map[schema.GroupVersionKind]bool)
	subresourceMap := make(map[schema.GroupVersionKind]Subresource)
	for _, kind := range rule.MatchResources.Kinds {
		addGVKToResourceTypesMap(kind, resourceTypesMap, subresourceMap, client)
	}

	if rule.MatchResources.Any != nil {
		for _, resFilter := range rule.MatchResources.Any {
			for _, kind := range resFilter.ResourceDescription.Kinds {
				addGVKToResourceTypesMap(kind, resourceTypesMap, subresourceMap, client)
			}
		}
	}

	if rule.MatchResources.All != nil {
		for _, resFilter := range rule.MatchResources.All {
			for _, kind := range resFilter.ResourceDescription.Kinds {
				addGVKToResourceTypesMap(kind, resourceTypesMap, subresourceMap, client)
			}
		}
	}
	return resourceTypesMap, subresourceMap
}

func addGVKToResourceTypesMap(kind string, resourceTypesMap map[schema.GroupVersionKind]bool, subresourceMap map[schema.GroupVersionKind]Subresource, client dclient.Interface) {
	gvString, k := kubeutils.GetKindFromGVK(kind)
	apiResource, parentApiResource, _, err := client.Discovery().FindResource(gvString, k)
	if err != nil {
		log.Log.Info("failed to find resource", "kind", kind, "error", err)
		return
	}

	// The resource is not a subresource
	if parentApiResource == nil {
		gvk := schema.GroupVersionKind{
			Group:   apiResource.Group,
			Version: apiResource.Version,
			Kind:    apiResource.Kind,
		}
		resourceTypesMap[gvk] = true
	} else {
		gvk := schema.GroupVersionKind{
			Group: apiResource.Group, Version: apiResource.Version, Kind: apiResource.Kind,
		}
		subresourceMap[gvk] = Subresource{
			APIResource:    *apiResource,
			ParentResource: *parentApiResource,
		}
	}
}
