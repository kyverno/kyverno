package match

import (
	"strings"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CheckKind checks if the resource kind matches the kinds in the policy. If the policy matches on subresources, then those resources are
// present in the subresourceGVKToAPIResource map. Set allowEphemeralContainers to true to allow ephemeral containers to be matched even when the
// policy does not explicitly match on ephemeral containers and only matches on pods.
func CheckKind(subresourceGVKToAPIResource map[string]*metav1.APIResource, kinds []string, gvk schema.GroupVersionKind, subresourceInAdmnReview string, allowEphemeralContainers bool) bool {
	result := false
	for _, k := range kinds {
		if k != "*" {
			gv, kind := kubeutils.GetKindFromGVK(k)
			apiResource, ok := subresourceGVKToAPIResource[k]
			if ok {
				result = apiResource.Group == gvk.Group && (apiResource.Version == gvk.Version || strings.Contains(gv, "*")) && apiResource.Kind == gvk.Kind
			} else { // if the kind is not found in the subresourceGVKToAPIResource, then it is not a subresource
				result = kind == gvk.Kind &&
					(subresourceInAdmnReview == "" ||
						(allowEphemeralContainers && subresourceInAdmnReview == "ephemeralcontainers"))
				if gv != "" {
					result = result && kubeutils.GroupVersionMatches(gv, gvk.GroupVersion().String())
				}
			}
		} else {
			result = true
		}
		if result {
			break
		}
	}
	return result
}
