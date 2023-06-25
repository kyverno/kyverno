package match

import (
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var podGVK = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

// CheckKind checks if the resource kind matches the kinds in the policy. If the policy matches on subresources, then those resources are
// present in the subresourceGVKToAPIResource map. Set allowEphemeralContainers to true to allow ephemeral containers to be matched even when the
// policy does not explicitly match on ephemeral containers and only matches on pods.
func CheckKind(kinds []string, gvk schema.GroupVersionKind, subresource string, allowEphemeralContainers bool) bool {
	for _, k := range kinds {
		group, version, kind, sub := kubeutils.ParseKindSelector(k)
		if wildcard.Match(group, gvk.Group) && wildcard.Match(version, gvk.Version) && wildcard.Match(kind, gvk.Kind) {
			if wildcard.Match(sub, subresource) {
				return true
			} else if allowEphemeralContainers && gvk == podGVK && subresource == "ephemeralcontainers" {
				return true
			}
		}
	}
	return false
}
