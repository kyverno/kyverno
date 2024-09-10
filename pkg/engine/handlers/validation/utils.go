package validation

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/utils/match"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func matchResource(resource unstructured.Unstructured, rule kyvernov1.Rule) bool {
	if rule.MatchResources.All != nil || rule.MatchResources.Any != nil {
		matched := match.CheckMatchesResources(
			resource,
			kyvernov2beta1.MatchResources{
				Any: rule.MatchResources.Any,
				All: rule.MatchResources.All,
			},
			make(map[string]string),
			kyvernov2.RequestInfo{},
			resource.GroupVersionKind(),
			"",
		)
		if matched != nil {
			return false
		}
	}
	if rule.ExcludeResources != nil {
		if rule.ExcludeResources.All != nil || rule.ExcludeResources.Any != nil {
			excluded := match.CheckMatchesResources(
				resource,
				kyvernov2beta1.MatchResources{
					Any: rule.ExcludeResources.Any,
					All: rule.ExcludeResources.All,
				},
				make(map[string]string),
				kyvernov2.RequestInfo{},
				resource.GroupVersionKind(),
				"",
			)
			if excluded == nil {
				return false
			}
		}
	}
	return true
}
