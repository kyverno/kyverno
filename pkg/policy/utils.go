package policy

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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
func isMatchResourcesAllValid(slice []string) bool {
	if len(slice) == 0 {
		return false
	}
	for i := 1; i < len(slice); i++ {
		if slice[i] != slice[0] {
			return false
		}
	}
	return true
}

func fetchUniqueKinds(rule kyverno.Rule) []string {
	var kindlist []string

	kindlist = append(kindlist, rule.MatchResources.Kinds...)

	for _, all := range rule.MatchResources.Any {
		kindlist = append(kindlist, all.Kinds...)
	}

	for _, all := range rule.MatchResources.All {
		kindlist = append(kindlist, all.Kinds...)
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

func constructUniquelist(ulists []unstructured.Unstructured) []*unstructured.Unstructured {
	inResult := make(map[*unstructured.Unstructured]bool)
	var result []*unstructured.Unstructured
	for _, list := range ulists {
		list := list
		if _, ok := inResult[&list]; !ok {
			inResult[&list] = true
			result = append(result, &list)
		}
	}
	return result
}
