package resource

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func FixupGenerateLabels(obj unstructured.Unstructured) {
	tidy := map[string]string{
		"app.kubernetes.io/managed-by": "kyverno",
	}
	if labels := obj.GetLabels(); labels != nil {
		for k, v := range labels {
			if !strings.HasPrefix(k, "generate.kyverno.io/") {
				tidy[k] = v
			}
		}
	}
	obj.SetLabels(tidy)
}
