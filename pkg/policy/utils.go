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

func fetchUniqueSpec(specs []*kyverno.ResourceSpec) []*kyverno.ResourceSpec {
	inResult := make(map[*kyverno.ResourceSpec]bool)
	var result []*kyverno.ResourceSpec
	for _, spec := range specs {
		if _, ok := inResult[spec]; !ok {
			inResult[spec] = true
			result = append(result, spec)
		}
	}
	return result
}

func constructUniquelist(ulists []unstructured.Unstructured) []unstructured.Unstructured {
	inResult := make(map[*unstructured.Unstructured]bool)
	var result []unstructured.Unstructured
	for _, list := range ulists {
		list := list
		if _, ok := inResult[&list]; !ok {
			inResult[&list] = true
			result = append(result, list)
		}
	}
	return result
}
