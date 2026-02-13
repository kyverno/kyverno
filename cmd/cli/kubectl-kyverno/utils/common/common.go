package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	crdscheme "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/scheme"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cli/loader"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/restmapper"
)

// GetResourceAccordingToResourcePath - get resources according to the resource path
func GetResourceAccordingToResourcePath(
	out io.Writer,
	fs billy.Filesystem,
	resourcePaths []string,
	cluster bool,
	policies []engineapi.GenericPolicy,
	dClient dclient.Interface,
	namespace string,
	policyReport bool,
	clusterWideResources bool,
	policyResourcePath string,
	resourceOptions loader.ResourceOptions,
	showPerformance bool,
) (resources []*unstructured.Unstructured, err error) {
	if fs != nil {
		resources, err = GetResourcesWithTest(out, fs, resourcePaths, policyResourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to extract the resources (%w)", err)
		}
	} else {
		if len(resourcePaths) > 0 && resourcePaths[0] == "-" {
			if source.IsStdin(resourcePaths[0]) {
				resourceStr := ""
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					resourceStr = resourceStr + scanner.Text() + "\n"
				}

				yamlBytes := []byte(resourceStr)
				resources, err = resource.GetUnstructuredResources(yamlBytes)
				if err != nil {
					return nil, fmt.Errorf("failed to extract the resources (%w)", err)
				}
			}
		} else {
			if len(resourcePaths) > 0 {
				fileDesc, err := os.Stat(resourcePaths[0])
				if err != nil {
					return nil, err
				}
				if fileDesc.IsDir() {
					files, err := os.ReadDir(resourcePaths[0])
					if err != nil {
						return nil, fmt.Errorf("failed to parse %v (%w)", resourcePaths[0], err)
					}
					listOfFiles := make([]string, 0)
					for _, file := range files {
						ext := filepath.Ext(file.Name())
						if ext == ".yaml" || ext == ".yml" {
							listOfFiles = append(listOfFiles, filepath.Join(resourcePaths[0], file.Name()))
						}
					}
					resourcePaths = listOfFiles
				}
			}
			if clusterWideResources {
				fetcher := &ResourceFetcher{
					Out:                  out,
					Policies:             policies,
					ResourcePaths:        resourcePaths,
					Client:               dClient,
					Cluster:              cluster,
					Namespace:            "",
					PolicyReport:         policyReport,
					ClusterWideResources: clusterWideResources,
					ResourceOptions:      resourceOptions,
					ShowPerformance:      showPerformance,
				}
				resources, err := fetcher.GetResources()
				if err != nil {
					return resources, err
				}
				if namespace == "" {
					return resources, nil
				}
			}
			fetcher := &ResourceFetcher{
				Out:                  out,
				Policies:             policies,
				ResourcePaths:        resourcePaths,
				Client:               dClient,
				Cluster:              cluster,
				Namespace:            namespace,
				PolicyReport:         policyReport,
				ClusterWideResources: false,
				ResourceOptions:      resourceOptions,
				ShowPerformance:      showPerformance,
			}
			namespaceResources, err := fetcher.GetResources()
			if err != nil {
				return resources, err
			}
			resources = append(resources, namespaceResources...)
		}
	}
	return resources, err
}

func GetKindsFromPolicy(out io.Writer, policy kyvernov1.PolicyInterface, subresources []v1alpha1.Subresource, dClient dclient.Interface) sets.Set[string] {
	knownkinds := sets.New[string]()
	for _, rule := range autogen.Default.ComputeRules(policy, "") {
		for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
			k, err := getKind(kind, subresources, dClient)
			if err != nil {
				fmt.Fprintf(out, "Error: %s", err.Error())
				continue
			}
			knownkinds.Insert(k)
		}
		if rule.ExcludeResources != nil {
			for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
				k, err := getKind(kind, subresources, dClient)
				if err != nil {
					fmt.Fprintf(out, "Error: %s", err.Error())
					continue
				}
				knownkinds.Insert(k)
			}
		}
	}
	return knownkinds
}

func getKind(kind string, subresources []v1alpha1.Subresource, dClient dclient.Interface) (string, error) {
	group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
	if subresource == "" {
		return kind, nil
	}
	if dClient == nil {
		gv := schema.GroupVersion{Group: group, Version: version}
		return getSubresourceKind(gv.String(), kind, subresource, subresources)
	}
	gvrss, err := dClient.Discovery().FindResources(group, version, kind, subresource)
	if err != nil {
		return kind, err
	}
	if len(gvrss) != 1 {
		return kind, fmt.Errorf("no unique match for kind %s", kind)
	}
	for _, api := range gvrss {
		return api.Kind, nil
	}
	return kind, nil
}

func getSubresourceKind(groupVersion, parentKind, subresourceName string, subresources []v1alpha1.Subresource) (string, error) {
	for _, subresource := range subresources {
		parentResourceGroupVersion := metav1.GroupVersion{
			Group:   subresource.ParentResource.Group,
			Version: subresource.ParentResource.Version,
		}.String()
		if groupVersion == "" || kubeutils.GroupVersionMatches(groupVersion, parentResourceGroupVersion) {
			if parentKind == subresource.ParentResource.Kind {
				if strings.ToLower(subresourceName) == strings.Split(subresource.Subresource.Name, "/")[1] {
					return subresource.Subresource.Kind, nil
				}
			}
		}
	}
	return "", fmt.Errorf("subresource %s not found for parent resource %s", subresourceName, parentKind)
}

func GetGitBranchOrPolicyPaths(gitBranch, repoURL string, policyPaths ...string) (string, string) {
	var gitPathToYamls string
	if gitBranch == "" {
		gitPathToYamls = "/"
		if string(policyPaths[0][len(policyPaths[0])-1]) == "/" {
			gitBranch = strings.ReplaceAll(policyPaths[0], repoURL+"/", "")
		} else {
			gitBranch = strings.ReplaceAll(policyPaths[0], repoURL, "")
		}
		if gitBranch == "" {
			gitBranch = "main"
		} else if string(gitBranch[0]) == "/" {
			gitBranch = gitBranch[1:]
		}
		return gitBranch, gitPathToYamls
	}
	if string(policyPaths[0][len(policyPaths[0])-1]) == "/" {
		gitPathToYamls = strings.ReplaceAll(policyPaths[0], repoURL+"/", "/")
	} else {
		gitPathToYamls = strings.ReplaceAll(policyPaths[0], repoURL, "/")
	}
	return gitBranch, gitPathToYamls
}

// ReadFile reads a file from either a billy.Filesystem or the local filesystem.
func ReadFile(f billy.Filesystem, filepath string) ([]byte, error) {
	if f != nil {
		file, err := f.Open(filepath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}
	return os.ReadFile(filepath)
}

// LoadYAML loads a YAML file and unmarshals it into a value of type T.
// T must be a pointer type that can be unmarshaled from YAML.
func LoadYAML[T any](f billy.Filesystem, filepath string, newInstance func() T) (T, error) {
	var zero T
	yamlBytes, err := ReadFile(f, filepath)
	if err != nil {
		return zero, err
	}
	vals := newInstance()
	if err := yaml.UnmarshalStrict(yamlBytes, vals); err != nil {
		return zero, err
	}
	return vals, nil
}

func LoadCrdFromPath(path string) error {
	absPath, err := getCrdPath(path)
	if err != nil {
		return err
	}
	crdscheme.Setup()
	crd, err := readCRDFromFile(absPath)
	if err != nil {
		return err
	}
	apiGroupResource := apiGroupResourcesFromCRD(crd)
	genericImageExtractors := GenerateImageExtractorsForCRD(crd)
	addGenericImageExtractor(genericImageExtractors)

	if err = addResourceGroup(apiGroupResource); err != nil {
		return err
	}

	return nil
}

func getCrdPath(path string) (string, error) {
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	ext := filepath.Ext(absPath)
	if ext != ".yaml" && ext != ".yml" {
		return "", fmt.Errorf("CRD file must have .yaml or .yml extension: %s", absPath)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("CRD file does not exist at path: %s", absPath)
	}
	return absPath, nil
}

func readCRDFromFile(path string) (*apiv1.CustomResourceDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	jsonData, err := yaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, _, err := crdscheme.Decoder.Decode(jsonData, nil, nil)
	if err != nil {
		return nil, err
	}
	crd, ok := obj.(*apiv1.CustomResourceDefinition)
	if !ok {
		return nil, fmt.Errorf("decoded object is not a CRD")
	}

	return crd, nil
}

func apiGroupResourcesFromCRD(crd *apiv1.CustomResourceDefinition) *restmapper.APIGroupResources {
	versionedResources := make(map[string][]metav1.APIResource)
	var preferredVersion string

	// Find preferred version (storage: true)
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			preferredVersion = v.Name
			break
		}
	}

	// For each served version, build resource list (including subresources)
	for _, v := range crd.Spec.Versions {
		if !v.Served {
			continue
		}
		var resources []metav1.APIResource

		// Main resource
		resources = append(resources, metav1.APIResource{
			Name:         crd.Spec.Names.Plural,
			SingularName: crd.Spec.Names.Singular,
			Namespaced:   crd.Spec.Scope == apiv1.NamespaceScoped,
			Kind:         crd.Spec.Names.Kind,
			Verbs:        metav1.Verbs{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"},
			Group:        crd.Spec.Group,
			Version:      v.Name,
		})

		// Subresources
		if v.Subresources != nil && v.Subresources.Status != nil {
			resources = append(resources, metav1.APIResource{
				Name:         crd.Spec.Names.Plural + "/status",
				SingularName: crd.Spec.Names.Singular,
				Namespaced:   crd.Spec.Scope == apiv1.NamespaceScoped,
				Kind:         crd.Spec.Names.Kind,
				Verbs:        metav1.Verbs{"get", "update", "patch"},
				Group:        crd.Spec.Group,
				Version:      v.Name,
			})
		}
		if v.Subresources != nil && v.Subresources.Scale != nil {
			resources = append(resources, metav1.APIResource{
				Name:         crd.Spec.Names.Plural + "/scale",
				SingularName: crd.Spec.Names.Singular,
				Namespaced:   crd.Spec.Scope == apiv1.NamespaceScoped,
				Kind:         "Scale",
				Verbs:        metav1.Verbs{"get", "update", "patch"},
				Group:        crd.Spec.Group,
				Version:      v.Name,
			})
		}

		versionedResources[v.Name] = resources
	}

	// Build APIGroup
	group := metav1.APIGroup{
		Name:     crd.Spec.Group,
		Versions: []metav1.GroupVersionForDiscovery{},
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: crd.Spec.Group + "/" + preferredVersion,
			Version:      preferredVersion,
		},
	}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			group.Versions = append(group.Versions, metav1.GroupVersionForDiscovery{
				GroupVersion: crd.Spec.Group + "/" + v.Name,
				Version:      v.Name,
			})
		}
	}

	return &restmapper.APIGroupResources{
		Group:              group,
		VersionedResources: versionedResources,
	}
}

func addResourceGroup(resource *restmapper.APIGroupResources) error {
	processor := data.GetProcessor()
	if processor == nil {
		panic("adding a resource group to a nil crd processor. exiting")
	}
	processor.AddResourceGroup(resource)
	return nil
}

func GenerateImageExtractorsForCRD(crd *apiv1.CustomResourceDefinition) []policiesv1alpha1.ImageExtractor {
	extractors := make([]policiesv1alpha1.ImageExtractor, 0, len(crd.Spec.Versions))

	for _, version := range crd.Spec.Versions {
		if version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
			continue
		}
		specSchema := version.Schema.OpenAPIV3Schema.Properties["spec"]
		imagePaths := extractImagePathsFromSpec(&specSchema, "spec")
		extractors = append(extractors, buildImageExtractors(imagePaths)...)
	}

	return extractors
}

func extractImagePathsFromSpec(schema *apiv1.JSONSchemaProps, prefix string) []string {
	var paths []string

	if schema.Type == "array" && schema.Items != nil && schema.Items.Schema != nil {
		if imageProp, ok := schema.Items.Schema.Properties["image"]; ok && imageProp.Type == "string" {
			fieldName := baseFieldName(prefix)
			if fieldName == "containers" || fieldName == "initContainers" || fieldName == "ephemeralContainers" {
				paths = append(paths, prefix)
			}
		}
	}

	if schema.Type == "object" {
		for name, prop := range schema.Properties {
			subPrefix := prefix + "." + name
			paths = append(paths, extractImagePathsFromSpec(&prop, subPrefix)...)
		}
	}

	return paths
}

func buildImageExtractors(paths []string) []policiesv1alpha1.ImageExtractor {
	extractors := make([]policiesv1alpha1.ImageExtractor, 0, len(paths))

	for _, path := range paths {
		safeNav := toSafeNavigation(path)
		expr := fmt.Sprintf("(object != null ? object : oldObject).%s.orValue([]).map(e, e.image)", safeNav)
		extractors = append(extractors, policiesv1alpha1.ImageExtractor{
			Name:       baseFieldName(path),
			Expression: expr,
		})
	}

	return extractors
}

func toSafeNavigation(path string) string {
	parts := strings.Split(path, ".")
	for i := range parts {
		if i > 0 { // first is "spec", leave as-is
			parts[i] = "?" + parts[i]
		}
	}
	return strings.Join(parts, ".")
}

func baseFieldName(path string) string {
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

func addGenericImageExtractor(genericImageExtractor []policiesv1alpha1.ImageExtractor) {
	compiler.SetGenericExtractors(genericImageExtractor)
}
