package policy

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

//Contains Check if strint is contained in a list of string
func containString(list []string, element string) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}

func isRunningPod(obj unstructured.Unstructured) bool {
	objMap := obj.UnstructuredContent()
	phase, ok, err := unstructured.NestedString(objMap, "status", "phase")
	if !ok || err != nil {
		return false
	}

	return phase == "Running"
}
