package common

import (
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// IsFilteredByConfig reports whether obj should be skipped per ConfigMap resourceFilters.
// Semantics match admission webhook filtering (pkg/webhooks/handlers/filter.go).
func IsFilteredByConfig(configuration config.Configuration, obj *unstructured.Unstructured) bool {
	if obj == nil || obj.Object == nil {
		return false
	}
	gvk := obj.GroupVersionKind()
	return configuration.ToFilter(gvk, "", obj.GetNamespace(), obj.GetName())
}
