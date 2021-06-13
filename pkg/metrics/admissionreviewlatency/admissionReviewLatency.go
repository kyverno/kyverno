package admissionreviewlatency

import (
	"fmt"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
	"time"
)

func (pm PromMetrics) registerAdmissionReviewLatencyMetric(
	clusterPoliciesCount, namespacedPoliciesCount int,
	validateRulesCount, mutateRulesCount, generateRulesCount int,
	resourceName, resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
	admissionRequestLatency float64,
	admissionRequestTimestamp int64,
) error {
	pm.AdmissionReviewLatency.With(prom.Labels{
		"cluster_policies_count":      fmt.Sprintf("%d", clusterPoliciesCount),
		"namespaced_policies_count":   fmt.Sprintf("%d", namespacedPoliciesCount),
		"validate_rules_count":        fmt.Sprintf("%d", validateRulesCount),
		"mutate_rules_count":          fmt.Sprintf("%d", mutateRulesCount),
		"generate_rules_count":        fmt.Sprintf("%d", generateRulesCount),
		"resource_name":               resourceName,
		"resource_kind":               resourceKind,
		"resource_namespace":          resourceNamespace,
		"resource_request_operation":  string(resourceRequestOperation),
		"admission_request_timestamp": fmt.Sprintf("%+v", time.Unix(admissionRequestTimestamp, 0)),
	}).Set(admissionRequestLatency)
	return nil
}

func (pm PromMetrics) ProcessEngineResponses(engineResponses []*response.EngineResponse, triggeredPolicies []kyverno.ClusterPolicy, admissionReviewLatencyDuration int64, resourceRequestOperation metrics.ResourceRequestOperation, admissionRequestTimestamp int64, logger logr.Logger) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(fmt.Errorf("panic initiated"), "error occurred while registering kyverno_admission_review_latency_milliseconds metrics")
		}
	}()
	if len(engineResponses) == 0 {
		return nil
	}
	resourceName, resourceNamespace, resourceKind := engineResponses[0].PolicyResponse.Resource.Name, engineResponses[0].PolicyResponse.Resource.Namespace, engineResponses[0].PolicyResponse.Resource.Kind
	clusterPoliciesCount, namespacedPoliciesCount, totalValidateRulesCount, totalMutateRulesCount, totalGenerateRulesCount := 0, 0, 0, 0, 0
	for i, e := range engineResponses {
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
		if triggeredPolicies[i].Namespace == "" {
			clusterPoliciesCount++
		} else {
			namespacedPoliciesCount++
		}
		totalValidateRulesCount += validateRulesCount
		totalMutateRulesCount += mutateRulesCount
		totalGenerateRulesCount += generateRulesCount
	}
	if totalValidateRulesCount+totalMutateRulesCount+totalGenerateRulesCount == 0 {
		return nil
	}
	admissionReviewLatencyDurationInMs := float64(admissionReviewLatencyDuration) / float64(1000*1000)
	return pm.registerAdmissionReviewLatencyMetric(clusterPoliciesCount, namespacedPoliciesCount, totalValidateRulesCount, totalMutateRulesCount, totalGenerateRulesCount, resourceName, resourceKind, resourceNamespace, resourceRequestOperation, admissionReviewLatencyDurationInMs, admissionRequestTimestamp)
}
