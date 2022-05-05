package clusterpolicy

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

func findResource(client discovery.DiscoveryInterface, apiVersion string, kind string) (*metav1.APIResource, *schema.GroupVersionResource, error) {
	var serverResources []*metav1.APIResourceList
	var err error
	if apiVersion == "" {
		serverResources, err = client.ServerPreferredResources()
	} else {
		_, serverResources, err = client.ServerGroupsAndResources()
	}
	if err != nil {
		logger.Error(err, "failed to find preferred resource version")
		return nil, nil, err
	}
	for _, serverResource := range serverResources {
		if apiVersion != "" && serverResource.GroupVersion != apiVersion {
			continue
		}
		for _, resource := range serverResource.APIResources {
			if strings.Contains(resource.Name, "/") {
				// skip the sub-resources like deployment/status
				continue
			}
			// match kind or names (e.g. Namespace, namespaces, namespace)
			// to allow matching API paths (e.g. /api/v1/namespaces).
			if resource.Kind == kind || resource.Name == kind || resource.SingularName == kind {
				gv, err := schema.ParseGroupVersion(serverResource.GroupVersion)
				if err != nil {
					logger.Error(err, "failed to parse groupVersion", "groupVersion", serverResource.GroupVersion)
					return nil, nil, err
				}
				gvr := gv.WithResource(resource.Name)
				return &resource, &gvr, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("kind '%s' not found in apiVersion '%s'", kind, apiVersion)
}

func getRule(client discovery.DiscoveryInterface, rule kyvernov1.Rule, updateValidate bool) ([]string, []string, []string) {
	var matchedGVK []string
	// matching kinds in generate policies need to be added to both webhook
	if rule.HasGenerate() {
		matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
		matchedGVK = append(matchedGVK, rule.Generation.ResourceSpec.Kind)
	}
	if (updateValidate && rule.HasValidate() || rule.HasImagesValidationChecks()) ||
		(updateValidate && rule.HasMutate() && rule.IsMutateExisting()) ||
		(!updateValidate && rule.HasMutate()) && !rule.IsMutateExisting() ||
		(!updateValidate && rule.HasVerifyImages()) {
		matchedGVK = append(matchedGVK, rule.MatchResources.GetKinds()...)
	}
	gvkMap := map[string]int{}
	gvrList := []schema.GroupVersionResource{}
	for _, gvk := range matchedGVK {
		if _, ok := gvkMap[gvk]; !ok {
			gvkMap[gvk] = 1
			// note: webhook stores GVR in its rules while policy stores GVK in its rules definition
			gv, k := kubeutils.GetKindFromGVK(gvk)
			switch k {
			case "Binding":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/binding"})
			case "NodeProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes/proxy"})
			case "PodAttachOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/attach"})
			case "PodExecOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/exec"})
			case "PodPortForwardOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/portforward"})
			case "PodProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods/proxy"})
			case "ServiceProxyOptions":
				gvrList = append(gvrList, schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services/proxy"})
			default:
				_, gvr, err := findResource(client, gv, k)
				if err != nil {
					logger.Error(err, "unable to convert GVK to GVR", "GVK", gvk)
					continue
				}
				if strings.Contains(gvk, "*") {
					gvrList = append(gvrList, schema.GroupVersionResource{Group: gvr.Group, Version: "*", Resource: gvr.Resource})
				} else {
					gvrList = append(gvrList, *gvr)
				}
			}
		}
	}
	var groups, versions, resources []string
	for _, gvr := range gvrList {
		groups = append(groups, gvr.Group)
		versions = append(versions, gvr.Version)
		resources = append(resources, gvr.Resource)
	}
	if utils.ContainsString(resources, "pods") {
		resources = append(resources, "pods/ephemeralcontainers")
	}
	if utils.ContainsString(resources, "services") {
		resources = append(resources, "services/status")
	}
	return removeDuplicates(groups), removeDuplicates(versions), removeDuplicates(resources)
}

func removeDuplicates(items []string) (res []string) {
	return sets.NewString(items...).List()
}
