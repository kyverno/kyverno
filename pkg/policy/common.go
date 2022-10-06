package policy

import (
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func transformResource(resource unstructured.Unstructured) []byte {
	data, err := resource.MarshalJSON()
	if err != nil {
		logging.Error(err, "failed to marshal resource")
		return nil
	}
	return data
}

func ParseNamespacedPolicy(key string) (string, string, bool) {
	namespace := ""
	index := strings.Index(key, "/")
	if index != -1 {
		namespace = key[:index]
		key = key[index+1:]
		return namespace, key, true
	}
	return namespace, key, false
}

// MergeResources merges b into a map
func MergeResources(a, b map[string]unstructured.Unstructured) {
	for k, v := range b {
		a[k] = v
	}
}

func (pc *PolicyController) getResourceList(kind, namespace string, labelSelector *metav1.LabelSelector, log logr.Logger) *unstructured.UnstructuredList {
	_, k := kubeutils.GetKindFromGVK(kind)
	resourceList, err := pc.client.ListResource("", k, namespace, labelSelector)
	if err != nil {
		log.Error(err, "failed to list resources", "kind", k, "namespace", namespace)
		return nil
	}
	return resourceList
}

// GetResourcesPerNamespace returns
// - Namespaced resources across all namespaces if namespace is set to empty "", for Namespaced Kind
// - Namespaced resources in the given namespace
// - Cluster-wide resources for Cluster-wide Kind
func (pc *PolicyController) getResourcesPerNamespace(kind string, namespace string, rule kyvernov1.Rule, log logr.Logger) map[string]unstructured.Unstructured {
	resourceMap := map[string]unstructured.Unstructured{}

	if kind == "Namespace" {
		namespace = ""
	}

	list := pc.getResourceList(kind, namespace, rule.MatchResources.Selector, log)
	if list != nil {
		for _, r := range list.Items {
			if pc.match(r, rule) {
				resourceMap[string(r.GetUID())] = r
			}
		}
	}

	// skip resources to be filtered
	excludeResources(resourceMap, rule.ExcludeResources.ResourceDescription, pc.configHandler, log)
	return resourceMap
}

func (pc *PolicyController) match(r unstructured.Unstructured, rule kyvernov1.Rule) bool {
	if r.GetDeletionTimestamp() != nil {
		return false
	}

	if r.GetKind() == "Pod" {
		if !isRunningPod(r) {
			return false
		}
	}

	// match name
	if rule.MatchResources.Name != "" {
		if !wildcard.Match(rule.MatchResources.Name, r.GetName()) {
			return false
		}
	}
	// Skip the filtered resources
	if pc.configHandler.ToFilter(r.GetKind(), r.GetNamespace(), r.GetName()) {
		return false
	}

	return true
}

// ExcludeResources ...
func excludeResources(included map[string]unstructured.Unstructured, exclude kyvernov1.ResourceDescription, configHandler config.Configuration, log logr.Logger) {
	if reflect.DeepEqual(exclude, (kyvernov1.ResourceDescription{})) {
		return
	}
	excludeName := func(name string) Condition {
		if exclude.Name == "" {
			return NotEvaluate
		}
		if wildcard.Match(exclude.Name, name) {
			return Skip
		}
		return Process
	}

	excludeNamespace := func(namespace string) Condition {
		if len(exclude.Namespaces) == 0 {
			return NotEvaluate
		}
		if utils.ContainsNamepace(exclude.Namespaces, namespace) {
			return Skip
		}
		return Process
	}

	excludeSelector := func(labelsMap map[string]string) Condition {
		if exclude.Selector == nil {
			return NotEvaluate
		}
		selector, err := metav1.LabelSelectorAsSelector(exclude.Selector)
		// if the label selector is incorrect, should be fail or
		if err != nil {
			log.Error(err, "failed to build label selector")
			return Skip
		}
		if selector.Matches(labels.Set(labelsMap)) {
			return Skip
		}
		return Process
	}

	findKind := func(kind string, kinds []string) bool {
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}

	excludeKind := func(kind string) Condition {
		if len(exclude.Kinds) == 0 {
			return NotEvaluate
		}

		if findKind(kind, exclude.Kinds) {
			return Skip
		}

		return Process
	}

	// check exclude condition for each resource
	for uid, resource := range included {
		// 0 -> don't check
		// 1 -> is not to be exclude
		// 2 -> to be exclude
		excludeEval := []Condition{}

		if ret := excludeName(resource.GetName()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		if ret := excludeNamespace(resource.GetNamespace()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		if ret := excludeSelector(resource.GetLabels()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		if ret := excludeKind(resource.GetKind()); ret != NotEvaluate {
			excludeEval = append(excludeEval, ret)
		}
		// exclude the filtered resources
		if configHandler.ToFilter(resource.GetKind(), resource.GetNamespace(), resource.GetName()) {
			delete(included, uid)
			continue
		}

		func() {
			for _, ret := range excludeEval {
				if ret == Process {
					// Process the resources
					continue
				}
				// Skip the resource from processing
				delete(included, uid)
			}
		}()
	}
}
