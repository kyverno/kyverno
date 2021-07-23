package admissionreviewduration

import (
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func (pm PromMetrics) registerAdmissionReviewDurationMetric(
	resourceName, resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
	admissionRequestLatency float64,
) error {
	pm.AdmissionReviewDuration.With(prom.Labels{
		"resource_name":              resourceName,
		"resource_kind":              resourceKind,
		"resource_namespace":         resourceNamespace,
		"resource_request_operation": string(resourceRequestOperation),
	}).Observe(admissionRequestLatency)
	return nil
}

func (pm PromMetrics) ProcessEngineResponses(engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64, resourceRequestOperation metrics.ResourceRequestOperation) error {
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
	admissionReviewLatencyDurationInSeconds := float64(admissionReviewLatencyDuration) / float64(1000*1000*1000)
	return pm.registerAdmissionReviewDurationMetric(resourceName, resourceKind, resourceNamespace, resourceRequestOperation, admissionReviewLatencyDurationInSeconds)
}
