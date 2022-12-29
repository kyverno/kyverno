package policy

// import (
// 	"reflect"

// 	"github.com/go-logr/logr"
// 	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
// 	"github.com/kyverno/kyverno/pkg/config"
// 	"github.com/kyverno/kyverno/pkg/engine"
// 	"github.com/kyverno/kyverno/pkg/utils/wildcard"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
// 	"k8s.io/apimachinery/pkg/labels"
// )

// // excludeAutoGenResources filter out the pods / jobs with ownerReference
// func excludeAutoGenResources(policy kyvernov1.PolicyInterface, resourceMap map[string]unstructured.Unstructured, log logr.Logger) {
// 	for uid, r := range resourceMap {
// 		if engine.ManagedPodResource(policy, r) {
// 			log.V(4).Info("exclude resource", "namespace", r.GetNamespace(), "kind", r.GetKind(), "name", r.GetName())
// 			delete(resourceMap, uid)
// 		}
// 	}
// }

// // ExcludeResources ...
// func excludeResources(included map[string]unstructured.Unstructured, exclude kyvernov1.ResourceDescription, configHandler config.Configuration, log logr.Logger) {
// 	if reflect.DeepEqual(exclude, (kyvernov1.ResourceDescription{})) {
// 		return
// 	}
// 	excludeName := func(name string) Condition {
// 		if exclude.Name == "" {
// 			return NotEvaluate
// 		}
// 		if wildcard.Match(exclude.Name, name) {
// 			return Skip
// 		}
// 		return Process
// 	}

// 	excludeNamespace := func(namespace string) Condition {
// 		if len(exclude.Namespaces) == 0 {
// 			return NotEvaluate
// 		}
// 		if wildcard.CheckPatterns(exclude.Namespaces, namespace) {
// 			return Skip
// 		}
// 		return Process
// 	}

// 	excludeSelector := func(labelsMap map[string]string) Condition {
// 		if exclude.Selector == nil {
// 			return NotEvaluate
// 		}
// 		selector, err := metav1.LabelSelectorAsSelector(exclude.Selector)
// 		// if the label selector is incorrect, should be fail or
// 		if err != nil {
// 			log.Error(err, "failed to build label selector")
// 			return Skip
// 		}
// 		if selector.Matches(labels.Set(labelsMap)) {
// 			return Skip
// 		}
// 		return Process
// 	}

// 	findKind := func(kind string, kinds []string) bool {
// 		for _, k := range kinds {
// 			if k == kind {
// 				return true
// 			}
// 		}
// 		return false
// 	}

// 	excludeKind := func(kind string) Condition {
// 		if len(exclude.Kinds) == 0 {
// 			return NotEvaluate
// 		}

// 		if findKind(kind, exclude.Kinds) {
// 			return Skip
// 		}

// 		return Process
// 	}

// 	// check exclude condition for each resource
// 	for uid, resource := range included {
// 		// 0 -> don't check
// 		// 1 -> is not to be exclude
// 		// 2 -> to be exclude
// 		excludeEval := []Condition{}

// 		if ret := excludeName(resource.GetName()); ret != NotEvaluate {
// 			excludeEval = append(excludeEval, ret)
// 		}
// 		if ret := excludeNamespace(resource.GetNamespace()); ret != NotEvaluate {
// 			excludeEval = append(excludeEval, ret)
// 		}
// 		if ret := excludeSelector(resource.GetLabels()); ret != NotEvaluate {
// 			excludeEval = append(excludeEval, ret)
// 		}
// 		if ret := excludeKind(resource.GetKind()); ret != NotEvaluate {
// 			excludeEval = append(excludeEval, ret)
// 		}
// 		// exclude the filtered resources
// 		if configHandler.ToFilter(resource.GetKind(), resource.GetNamespace(), resource.GetName()) {
// 			delete(included, uid)
// 			continue
// 		}

// 		func() {
// 			for _, ret := range excludeEval {
// 				if ret == Process {
// 					// Process the resources
// 					continue
// 				}
// 				// Skip the resource from processing
// 				delete(included, uid)
// 			}
// 		}()
// 	}
// }
