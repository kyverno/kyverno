package admissionrequests

import (
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func (pm PromMetrics) registerAdmissionRequestsMetric(
	resourceName, resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
) error {
	pm.AdmissionRequests.With(prom.Labels{
		"resource_name":              resourceName,
		"resource_kind":              resourceKind,
		"resource_namespace":         resourceNamespace,
		"resource_request_operation": string(resourceRequestOperation),
	}).Inc()
	return nil
}

func (pm PromMetrics) ProcessEngineResponses(engineResponses []*response.EngineResponse, resourceRequestOperation metrics.ResourceRequestOperation) error {
	if len(engineResponses) == 0 {
		return nil
	}
	resourceName, resourceNamespace, resourceKind := engineResponses[0].PolicyResponse.Resource.Name, engineResponses[0].PolicyResponse.Resource.Namespace, engineResponses[0].PolicyResponse.Resource.Kind
	totalValidateRulesCount, totalMutateRulesCount, totalGenerateRulesCount := 0, 0, 0
	for _, e := range engineResponses {
		validateRulesCount, mutateRulesCount, generateRulesCount := 0, 0, 0
		for _, rule := range e.PolicyResponse.Rules {
			switch rule.Type {
			case "Validation":
				validateRulesCount++
			case "Mutation":
				mutateRulesCount++
			case "Generation":
				generateRulesCount++
			}
		}
		// no rules triggered
		if validateRulesCount+mutateRulesCount+generateRulesCount == 0 {
			continue
		}

		totalValidateRulesCount += validateRulesCount
		totalMutateRulesCount += mutateRulesCount
		totalGenerateRulesCount += generateRulesCount
	}
	if totalValidateRulesCount+totalMutateRulesCount+totalGenerateRulesCount == 0 {
		return nil
	}
	return pm.registerAdmissionRequestsMetric(resourceName, resourceKind, resourceNamespace, resourceRequestOperation)
}
