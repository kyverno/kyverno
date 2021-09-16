package admissionreviewduration

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func (pc PromConfig) registerAdmissionReviewDurationMetric(
	resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
	admissionRequestLatency float64,
) error {
	includeNamespaces, excludeNamespaces := pc.Config.GetIncludeNamespaces(), pc.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && metrics.ElementInSlice(resourceNamespace, excludeNamespaces) {
		pc.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_admission_review_duration_seconds metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return nil
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !metrics.ElementInSlice(resourceNamespace, includeNamespaces) {
		pc.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_admission_review_duration_seconds metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
		return nil
	}
	pc.Metrics.AdmissionReviewDuration.With(prom.Labels{
		"resource_kind":              resourceKind,
		"resource_namespace":         resourceNamespace,
		"resource_request_operation": string(resourceRequestOperation),
	}).Observe(admissionRequestLatency)
	return nil
}

func (pc PromConfig) ProcessEngineResponses(engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64, resourceRequestOperation metrics.ResourceRequestOperation) error {
	if len(engineResponses) == 0 {
		return nil
	}
	resourceNamespace, resourceKind := engineResponses[0].PolicyResponse.Resource.Namespace, engineResponses[0].PolicyResponse.Resource.Kind
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
	return pc.registerAdmissionReviewDurationMetric(resourceKind, resourceNamespace, resourceRequestOperation, admissionReviewLatencyDurationInSeconds)
}
