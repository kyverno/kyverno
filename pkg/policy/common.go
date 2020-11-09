package policy

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/minio/minio/pkg/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func buildPolicyLabel(policyName string) (labels.Selector, error) {
	policyLabelmap := map[string]string{"policy": policyName}
	//NOt using a field selector, as the match function will have to cast the runtime.object
	// to get the field, while it can get labels directly, saves the cast effort
	ls := &metav1.LabelSelector{}
	if err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&policyLabelmap, ls, nil); err != nil {
		return nil, fmt.Errorf("failed to generate label sector of Policy name %s: %v", policyName, err)
	}
	policySelector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, fmt.Errorf("Policy %s has invalid label selector: %v", policyName, err)
	}
	return policySelector, nil
}

func transformResource(resource unstructured.Unstructured) []byte {
	data, err := resource.MarshalJSON()
	if err != nil {
		log.Log.Error(err, "failed to marshal resource")
		return nil
	}
	return data
}

// convertPoliciesToClusterPolicies - convert array of Policy to array of ClusterPolicy
func convertPoliciesToClusterPolicies(nsPolicies []*kyverno.Policy) []*kyverno.ClusterPolicy {
	var cpols []*kyverno.ClusterPolicy
	for _, pol := range nsPolicies {
		cpol := kyverno.ClusterPolicy(*pol)
		cpols = append(cpols, &cpol)
	}
	return cpols
}

// ConvertPolicyToClusterPolicy - convert Policy to ClusterPolicy
func ConvertPolicyToClusterPolicy(nsPolicies *kyverno.Policy) *kyverno.ClusterPolicy {
	cpol := kyverno.ClusterPolicy(*nsPolicies)
	return &cpol
}

func parseNamespacedPolicy(key string) (string, string, bool) {
	namespace := ""
	index := strings.Index(key, "/")
	if index != -1 {
		namespace = key[:index]
		key = key[index+1:]
		return namespace, key, true
	}
	return namespace, key, false
}

// merge b into a map
func MergeResources(a, b map[string]unstructured.Unstructured) {
	for k, v := range b {
		a[k] = v
	}
}

// excludePod filter out the pods with ownerReference
func ExcludePod(resourceMap map[string]unstructured.Unstructured, log logr.Logger) map[string]unstructured.Unstructured {
	for uid, r := range resourceMap {
		if r.GetKind() != "Pod" {
			continue
		}

		if len(r.GetOwnerReferences()) > 0 {
			log.V(4).Info("exclude Pod", "namespace", r.GetNamespace(), "name", r.GetName())
			delete(resourceMap, uid)
		}
	}

	return resourceMap
}

func GetNamespacesForRule(rule *kyverno.Rule, nslister listerv1.NamespaceLister, log logr.Logger) []string {
	if len(rule.MatchResources.Namespaces) == 0 {
		return GetAllNamespaces(nslister, log)
	}

	var wildcards []string
	var results []string
	for _, nsName := range rule.MatchResources.Namespaces {
		if HasWildcard(nsName) {
			wildcards = append(wildcards, nsName)
		}

		results = append(results, nsName)
	}

	if len(wildcards) > 0 {
		wildcardMatches := GetMatchingNamespaces(wildcards, nslister, log)
		results = append(results, wildcardMatches...)
	}

	return results
}

func HasWildcard(s string) bool {
	if s == "" {
		return false
	}

	return strings.Contains(s, "*") || strings.Contains(s, "?")
}

func GetMatchingNamespaces(wildcards []string, nslister listerv1.NamespaceLister, log logr.Logger) []string {
	all := GetAllNamespaces(nslister, log)
	if len(all) == 0 {
		return all
	}

	var results []string
	for _, wc := range wildcards {
		for _, ns := range all {
			if wildcard.Match(wc, ns) {
				results = append(results, ns)
			}
		}
	}

	return results
}

func GetAllNamespaces(nslister listerv1.NamespaceLister, log logr.Logger) []string {
	var results []string
	namespaces, err := nslister.List(labels.NewSelector())
	if err != nil {
		log.Error(err, "Failed to list namespaces")
	}
	for _, n := range namespaces {
		name := n.GetName()
		results = append(results, name)
	}

	return results
}

func GetResourcesPerNamespace(kind string, client *client.Client, namespace string, rule kyverno.Rule, configHandler config.Interface, log logr.Logger) map[string]unstructured.Unstructured {
	resourceMap := map[string]unstructured.Unstructured{}
	ls := rule.MatchResources.Selector

	if kind == "Namespace" {
		namespace = ""
	}

	list, err := client.ListResource("", kind, namespace, ls)
	if err != nil {
		log.Error(err, "failed to list resources", "kind", kind, "namespace", namespace)
		return nil
	}
	// filter based on name
	for _, r := range list.Items {
		if r.GetDeletionTimestamp() != nil {
			continue
		}

		if r.GetKind() == "Pod" {
			if !isRunningPod(r) {
				continue
			}
		}

		// match name
		if rule.MatchResources.Name != "" {
			if !wildcard.Match(rule.MatchResources.Name, r.GetName()) {
				continue
			}
		}
		// Skip the filtered resources
		if configHandler.ToFilter(r.GetKind(), r.GetNamespace(), r.GetName()) {
			continue
		}

		//TODO check if the group version kind is present or not
		resourceMap[string(r.GetUID())] = r
	}

	// exclude the resources
	// skip resources to be filtered
	ExcludeResources(resourceMap, rule.ExcludeResources.ResourceDescription, configHandler, log)
	return resourceMap
}

func ExcludeResources(included map[string]unstructured.Unstructured, exclude kyverno.ResourceDescription, configHandler config.Interface, log logr.Logger) {
	if reflect.DeepEqual(exclude, (kyverno.ResourceDescription{})) {
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
		// 0 -> dont check
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

		func() bool {
			for _, ret := range excludeEval {
				if ret == Process {
					// Process the resources
					continue
				}
			}
			// Skip the resource from processing
			delete(included, uid)
			return false
		}()
	}
}
