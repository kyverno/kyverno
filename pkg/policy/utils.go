package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func fetchUniqueKinds(rule kyvernov1.Rule) []string {
	kinds := sets.New(rule.MatchResources.Kinds...)

	for _, any := range rule.MatchResources.Any {
		kinds.Insert(any.Kinds...)
	}

	for _, all := range rule.MatchResources.All {
		kinds.Insert(all.Kinds...)
	}

	return kinds.UnsortedList()
}

func convertlist(ulists []unstructured.Unstructured) []*unstructured.Unstructured {
	var result []*unstructured.Unstructured
	for _, list := range ulists {
		result = append(result, list.DeepCopy())
	}
	return result
}

func castPolicy(p interface{}) kyvernov1.PolicyInterface {
	var policy kyvernov1.PolicyInterface
	switch obj := p.(type) {
	case *kyvernov1.ClusterPolicy:
		policy = obj
	case *kyvernov1.Policy:
		policy = obj
	}
	return policy
}
