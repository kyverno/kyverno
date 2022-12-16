package engine

import (
	"strings"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetSubresourceGVKToAPIResourceMap returns a map of subresource GVK to APIResource. This is used to determine if a resource is a subresource.
func GetSubresourceGVKToAPIResourceMap(kindsInPolicy []string, ctx *PolicyContext) map[string]*metav1.APIResource {
	subresourceGVKToAPIResource := make(map[string]*metav1.APIResource)
	for _, gvk := range kindsInPolicy {
		gv, k := kubeutils.GetKindFromGVK(gvk)
		parentKind, subresource := kubeutils.SplitSubresource(k)
		// Len of subresources is non zero only when validation request was sent from CLI without connecting to the cluster.
		if len(ctx.subresourcesInPolicy) != 0 {
			if subresource != "" {
				for _, subresourceInPolicy := range ctx.subresourcesInPolicy {
					parentResourceGroupVersion := metav1.GroupVersion{
						Group:   subresourceInPolicy.ParentResource.Group,
						Version: subresourceInPolicy.ParentResource.Version,
					}.String()
					if gv == "" || kubeutils.GroupVersionMatches(gv, parentResourceGroupVersion) {
						if parentKind == subresourceInPolicy.ParentResource.Kind {
							if strings.ToLower(subresource) == strings.Split(subresourceInPolicy.APIResource.Name, "/")[1] {
								subresourceGVKToAPIResource[gvk] = &(subresourceInPolicy.APIResource)
								break
							}
						}
					}
				}
			} else { // Complete kind may be a subresource, for eg- 'PodExecOptions'
				for _, subresourceInPolicy := range ctx.subresourcesInPolicy {
					// Subresources which can be just specified by kind, for eg- 'PodExecOptions'
					// have different kind than their parent resource. Otherwise for subresources which
					// have same kind as parent resource, need to be specified as Kind/Subresource, eg - 'Pod/status'
					if k == subresourceInPolicy.APIResource.Kind &&
						k != subresourceInPolicy.ParentResource.Kind {
						subresourceGroupVersion := metav1.GroupVersion{
							Group:   subresourceInPolicy.APIResource.Group,
							Version: subresourceInPolicy.APIResource.Version,
						}.String()
						if gv == "" || kubeutils.GroupVersionMatches(gv, subresourceGroupVersion) {
							subresourceGVKToAPIResource[gvk] = subresourceInPolicy.APIResource.DeepCopy()
							break
						}
					}
				}
			}
		} else if ctx.client != nil {
			// find the resource from API client
			apiResource, _, _, err := ctx.client.Discovery().FindResource(gv, k)
			if err == nil {
				if kubeutils.IsSubresource(apiResource.Name) {
					subresourceGVKToAPIResource[gvk] = apiResource
				}
			}
		}
	}
	return subresourceGVKToAPIResource
}
