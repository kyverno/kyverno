package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RemoveDuplicateAndObjectVariables - remove duplicate variables
func RemoveDuplicateAndObjectVariables(matches [][]string) string {
	var variableStr string
	for _, m := range matches {
		for _, v := range m {
			foundVariable := strings.Contains(variableStr, v)
			if !foundVariable {
				if !strings.Contains(v, "request.object") && !strings.Contains(v, "element") && v == "elementIndex" {
					variableStr = variableStr + " " + v
				}
			}
		}
	}
	return variableStr
}

// GetResourceAccordingToResourcePath - get resources according to the resource path
func GetResourceAccordingToResourcePath(fs billy.Filesystem, resourcePaths []string,
	cluster bool, policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, dClient dclient.Interface, namespace string, policyReport bool, isGit bool, policyResourcePath string,
) (resources []*unstructured.Unstructured, err error) {
	if isGit {
		resources, err = GetResourcesWithTest(fs, policies, resourcePaths, isGit, policyResourcePath)
		if err != nil {
			return nil, sanitizederror.NewWithError("failed to extract the resources", err)
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
					return nil, sanitizederror.NewWithError("failed to extract the resources", err)
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
						return nil, sanitizederror.NewWithError(fmt.Sprintf("failed to parse %v", resourcePaths[0]), err)
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

			resources, err = GetResources(policies, validatingAdmissionPolicies, resourcePaths, dClient, cluster, namespace, policyReport)
			if err != nil {
				return resources, err
			}
		}
	}
	return resources, err
}

func GetKindsFromPolicy(policy kyvernov1.PolicyInterface, subresources []valuesapi.Subresource, dClient dclient.Interface) map[string]struct{} {
	kindOnwhichPolicyIsApplied := make(map[string]struct{})
	for _, rule := range autogen.ComputeRules(policy) {
		for _, kind := range rule.MatchResources.ResourceDescription.Kinds {
			k, err := getKind(kind, subresources, dClient)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
				continue
			}
			kindOnwhichPolicyIsApplied[k] = struct{}{}
		}
		for _, kind := range rule.ExcludeResources.ResourceDescription.Kinds {
			k, err := getKind(kind, subresources, dClient)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
				continue
			}
			kindOnwhichPolicyIsApplied[k] = struct{}{}
		}
	}
	return kindOnwhichPolicyIsApplied
}

func getKind(kind string, subresources []valuesapi.Subresource, dClient dclient.Interface) (string, error) {
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

func getSubresourceKind(groupVersion, parentKind, subresourceName string, subresources []valuesapi.Subresource) (string, error) {
	for _, subresource := range subresources {
		parentResourceGroupVersion := metav1.GroupVersion{
			Group:   subresource.ParentResource.Group,
			Version: subresource.ParentResource.Version,
		}.String()
		if groupVersion == "" || kubeutils.GroupVersionMatches(groupVersion, parentResourceGroupVersion) {
			if parentKind == subresource.ParentResource.Kind {
				if strings.ToLower(subresourceName) == strings.Split(subresource.APIResource.Name, "/")[1] {
					return subresource.APIResource.Kind, nil
				}
			}
		}
	}
	return "", sanitizederror.NewWithError(fmt.Sprintf("subresource %s not found for parent resource %s", subresourceName, parentKind), nil)
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
