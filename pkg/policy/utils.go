package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func isRunningPod(obj unstructured.Unstructured) bool {
	objMap := obj.UnstructuredContent()
	phase, ok, err := unstructured.NestedString(objMap, "status", "phase")
	if !ok || err != nil {
		return false
	}

	return phase == "Running"
}

// check if all slice elements are same
func isMatchResourcesAllValid(rule kyvernov1.Rule) bool {
	var kindlist []string
	if rule.MatchResourcesXXX != nil {
		for _, all := range rule.MatchResourcesXXX.All {
			kindlist = append(kindlist, all.Kinds...)
		}
	}

	if len(kindlist) == 0 {
		return false
	}

	for i := 1; i < len(kindlist); i++ {
		if kindlist[i] != kindlist[0] {
			return false
		}
	}
	return true
}

func fetchUniqueKinds(rule kyvernov1.Rule) []string {
	var kindlist []string
	if rule.MatchResourcesXXX != nil {
		kindlist = append(kindlist, rule.MatchResourcesXXX.Kinds...)

		for _, all := range rule.MatchResourcesXXX.Any {
			kindlist = append(kindlist, all.Kinds...)
		}

		if isMatchResourcesAllValid(rule) {
			for _, all := range rule.MatchResourcesXXX.All {
				kindlist = append(kindlist, all.Kinds...)
			}
		}
	}

	inResult := make(map[string]bool)
	var result []string
	for _, kind := range kindlist {
		if _, ok := inResult[kind]; !ok {
			inResult[kind] = true
			result = append(result, kind)
		}
	}
	return result
}

func convertlist(ulists []unstructured.Unstructured) []*unstructured.Unstructured {
	var result []*unstructured.Unstructured
	for _, list := range ulists {
		result = append(result, list.DeepCopy())
	}
	return result
}
