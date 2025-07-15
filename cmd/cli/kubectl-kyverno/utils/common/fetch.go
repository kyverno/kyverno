package common

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type resourceTypeInfo struct {
	gvkMap         map[schema.GroupVersionKind]bool
	subresourceMap map[schema.GroupVersionKind]v1alpha1.Subresource
}
type ResourceFetcher struct {
	Out                  io.Writer
	Policies             []engineapi.GenericPolicy
	ResourcePaths        []string
	Client               dclient.Interface
	Cluster              bool
	Namespace            string
	PolicyReport         bool
	ClusterWideResources bool
}

// GetResources gets matched resources by the given policies
// The resources are fetched from:
// - local paths, if given
// - the k8s cluster, if given
func (rf *ResourceFetcher) GetResources() ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	var err error

	if rf.Cluster && rf.Client != nil {
		resources, err = rf.getFromCluster()
		if err != nil {
			return resources, err
		}
	} else if len(rf.ResourcePaths) > 0 {
		resources, err = rf.getFromLocalFiles()
		if err != nil {
			return resources, err
		}
	}
	return resources, err
}

func (rf *ResourceFetcher) getFromLocalFiles() ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	for _, path := range rf.ResourcePaths {
		resourceBytes, err := resource.GetFileBytes(path)
		if err != nil {
			if rf.PolicyReport {
				log.Log.V(3).Info(fmt.Sprintf("failed to load resources: %s.", path), "error", err)
			} else {
				fmt.Fprintf(rf.Out, "\n----------------------------------------------------------------------\nfailed to load resources: %s. \nerror: %s\n----------------------------------------------------------------------\n", path, err)
			}
			continue
		}

		getResources, err := resource.GetUnstructuredResources(resourceBytes)
		if err != nil {
			return nil, err
		}

		resources = append(resources, getResources...)
	}
	return resources, nil
}

// getFromCluster fetches resources from the cluster.
// It will first extract the matched resources from the policies.
// Then it will fetch the resources from the cluster.
func (rf *ResourceFetcher) getFromCluster() ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	info := &resourceTypeInfo{
		gvkMap:         make(map[schema.GroupVersionKind]bool),
		subresourceMap: make(map[schema.GroupVersionKind]v1alpha1.Subresource),
	}
	// extract the matched resources from the policies.
	rf.extractResourcesFromPolicies(info)
	// fetch the resources from the cluster.
	resourceMap, err := rf.listResources(info)
	if err != nil {
		return nil, err
	}
	if len(rf.ResourcePaths) == 0 {
		for _, rr := range resourceMap {
			resources = append(resources, rr)
		}
	} else {
		for _, resourcePath := range rf.ResourcePaths {
			lenOfResource := len(resources)
			for rn, rr := range resourceMap {
				s := strings.Split(rn, "-")
				if s[2] == resourcePath {
					resources = append(resources, rr)
				}
			}
			if lenOfResource >= len(resources) {
				if rf.PolicyReport {
					log.Log.V(3).Info(fmt.Sprintf("%s not found in cluster", resourcePath))
				} else {
					fmt.Fprintf(rf.Out, "\n----------------------------------------------------------------------\nresource %s not found in cluster\n----------------------------------------------------------------------\n", resourcePath)
				}
				return nil, fmt.Errorf("%s not found in cluster", resourcePath)
			}
		}
	}
	return resources, nil
}

func (rf *ResourceFetcher) extractResourcesFromPolicies(info *resourceTypeInfo) {
	for _, policy := range rf.Policies {
		if kpol := policy.AsKyvernoPolicy(); kpol != nil {
			for _, rule := range autogen.Default.ComputeRules(kpol, "") {
				rf.getKindsFromRule(rule, info)
			}
		} else {
			var matchResources *admissionregistrationv1.MatchResources
			if vap := policy.AsValidatingAdmissionPolicy(); vap != nil {
				matchResources = vap.GetDefinition().Spec.MatchConstraints
			} else if vp := policy.AsValidatingPolicy(); vp != nil {
				matchResources = vp.Spec.MatchConstraints
			} else if ivp := policy.AsImageValidatingPolicy(); ivp != nil {
				matchResources = ivp.Spec.MatchConstraints
			} else if dp := policy.AsDeletingPolicy(); dp != nil {
				matchResources = dp.Spec.MatchConstraints
			} else if mapPolicy := policy.AsMutatingAdmissionPolicy(); mapPolicy != nil {
				converted := admissionpolicy.ConvertMatchResources(mapPolicy.GetDefinition().Spec.MatchConstraints)
				matchResources = converted
			} else if gpol := policy.AsGeneratingPolicy(); gpol != nil {
				matchResources = gpol.Spec.MatchConstraints
			}
			rf.getKindsFromPolicy(matchResources, info)
		}
	}
}

// getKindsFromRule will return the kinds from policy match block
func (rf *ResourceFetcher) getKindsFromRule(
	rule kyvernov1.Rule,
	info *resourceTypeInfo,
) {
	for _, kind := range rule.MatchResources.Kinds {
		rf.addToresourceTypeInfo(kind, info)
	}
	if rule.MatchResources.Any != nil {
		for _, resFilter := range rule.MatchResources.Any {
			for _, kind := range resFilter.ResourceDescription.Kinds {
				rf.addToresourceTypeInfo(kind, info)
			}
		}
	}
	if rule.MatchResources.All != nil {
		for _, resFilter := range rule.MatchResources.All {
			for _, kind := range resFilter.ResourceDescription.Kinds {
				rf.addToresourceTypeInfo(kind, info)
			}
		}
	}
}

// getKindsFromPolicy will return the kinds from the following policies match block:
// 1. K8s ValidatingAdmissionPolicy
// 2. K8s MutatingAdmissionPolicy
// 3. ValidatingPolicy
func (rf *ResourceFetcher) getKindsFromPolicy(
	matchResources *admissionregistrationv1.MatchResources,
	info *resourceTypeInfo,
) {
	restMapper, err := utils.GetRESTMapper(rf.Client, false)
	if err != nil {
		log.Log.V(3).Info("failed to get rest mapper", "error", err)
		return
	}
	kinds, err := admissionpolicy.GetKinds(matchResources, restMapper)
	if err != nil {
		log.Log.V(3).Info("failed to get kinds from validating admission policy", "error", err)
		return
	}
	for _, kind := range kinds {
		rf.addToresourceTypeInfo(kind, info)
	}
}

func (rf *ResourceFetcher) addToresourceTypeInfo(
	kind string,
	info *resourceTypeInfo,
) {
	group, version, kind, subresource := kubeutils.ParseKindSelector(kind)
	resourceDefs, err := rf.Client.Discovery().FindResources(group, version, kind, subresource)
	if err != nil {
		log.Log.V(2).Info("Failed to find resource", "kind", kind, "error", err)
		return
	}

	for parent, child := range resourceDefs {
		if rf.ClusterWideResources && child.Namespaced {
			continue
		}

		if parent.SubResource == "" {
			info.gvkMap[parent.GroupVersionKind()] = true
		} else {
			subGVK := schema.GroupVersionKind{
				Group:   child.Group,
				Version: child.Version,
				Kind:    child.Kind,
			}
			info.subresourceMap[subGVK] = v1alpha1.Subresource{
				Subresource: child,
				ParentResource: metav1.APIResource{
					Group:   parent.Group,
					Version: parent.Version,
					Kind:    parent.Kind,
					Name:    parent.Resource,
				},
			}
		}
	}
}

func (rf *ResourceFetcher) listResources(
	info *resourceTypeInfo,
) (map[string]*unstructured.Unstructured, error) {
	result := make(map[string]*unstructured.Unstructured)

	// list standard resources
	for gvk := range info.gvkMap {
		resourceList, err := rf.Client.ListResource(
			context.TODO(),
			gvk.GroupVersion().String(),
			gvk.Kind,
			rf.Namespace,
			nil,
		)
		if err != nil {
			log.Log.V(3).Info("failed to list resource", "gvk", gvk, "error", err)
			continue
		}
		for _, resource := range resourceList.Items {
			key := fmt.Sprintf("%s-%s-%s", gvk.Kind, resource.GetNamespace(), resource.GetName())
			resource.SetGroupVersionKind(gvk)
			result[key] = resource.DeepCopy()
		}
	}

	// list subresources
	for subGVK, subresource := range info.subresourceMap {
		parentGV := schema.GroupVersion{
			Group:   subresource.ParentResource.Group,
			Version: subresource.ParentResource.Version,
		}
		resourceList, err := rf.Client.ListResource(
			context.TODO(),
			parentGV.String(),
			subresource.ParentResource.Kind,
			rf.Namespace,
			nil,
		)
		if err != nil {
			log.Log.V(3).Info("failed to list parent resource", "gv", parentGV, "kind", subresource.ParentResource.Kind, "error", err)
			continue
		}

		for _, parent := range resourceList.Items {
			subresourceName := strings.Split(subresource.Subresource.Name, "/")[1]
			resource, err := rf.Client.GetResource(
				context.TODO(),
				parentGV.String(),
				subresource.ParentResource.Kind,
				rf.Namespace,
				parent.GetName(),
				subresourceName,
			)
			if err != nil {
				log.Log.V(3).Info("failed to get subresource", "parent", parent.GetName(), "subresource", subresourceName, "error", err)
				continue
			}

			key := fmt.Sprintf("%s-%s-%s", subGVK.Kind, resource.GetNamespace(), resource.GetName())
			resource.SetGroupVersionKind(subGVK)
			result[key] = resource.DeepCopy()
		}
	}
	return result, nil
}

// GetResourcesWithTest with gets matched resources by the given policies
func GetResourcesWithTest(out io.Writer, fs billy.Filesystem, resourcePaths []string, policyResourcePath string) ([]*unstructured.Unstructured, error) {
	resources := make([]*unstructured.Unstructured, 0)
	if len(resourcePaths) > 0 {
		for _, resourcePath := range resourcePaths {
			var resourceBytes []byte
			var err error
			if fs != nil {
				filep, err := fs.Open(filepath.Join(policyResourcePath, resourcePath))
				if err != nil {
					fmt.Fprintf(out, "Unable to open resource file: %s. error: %s", resourcePath, err)
					continue
				}
				resourceBytes, _ = io.ReadAll(filep)
			} else {
				resourceBytes, err = resource.GetFileBytes(resourcePath)
			}
			if err != nil {
				fmt.Fprintf(out, "\n----------------------------------------------------------------------\nfailed to load resources: %s. \nerror: %s\n----------------------------------------------------------------------\n", resourcePath, err)
				continue
			}

			getResources, err := resource.GetUnstructuredResources(resourceBytes)
			if err != nil {
				return nil, err
			}

			resources = append(resources, getResources...)
		}
	}
	return resources, nil
}
