package policy

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func isRunningPod(obj unstructured.Unstructured) bool {
	objMap := obj.UnstructuredContent()
	phase, ok, err := unstructured.NestedString(objMap, "status", "phase")
	if !ok || err != nil {
		return false
	}

	return phase == "Running"
}
