package utils

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionv1 "k8s.io/api/admission/v1"
)

// MatchDeleteOperation checks if the rule specifies the DELETE operation.
func MatchDeleteOperation(rule kyvernov1.Rule) bool {
	ops := rule.MatchResources.GetOperations()
	for _, rscFilters := range append(rule.MatchResources.All, rule.MatchResources.Any...) {
		ops = append(ops, rscFilters.ResourceDescription.GetOperations()...)
	}

	return datautils.SliceContains(ops, string(admissionv1.Delete))
}
