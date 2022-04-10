package admissionrequests

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
	prom "github.com/prometheus/client_golang/prometheus"
)

func registerAdmissionRequestsMetric(
	pc *metrics.PromConfig,
	m *metrics.MetricsConfig,
	resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
) error {
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && utils.ContainsString(excludeNamespaces, resourceNamespace) {
<<<<<<< HEAD
		metrics.Logger().Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return nil
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, resourceNamespace) {
		metrics.Logger().Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
=======
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return nil
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, resourceNamespace) {
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
>>>>>>> 4d3fab5be (metrics in otel format, created struct for binding data)
		return nil
	}
	pc.Metrics.AdmissionRequests.With(prom.Labels{
		"resource_kind":              resourceKind,
		"resource_namespace":         resourceNamespace,
		"resource_request_operation": string(resourceRequestOperation),
	}).Inc()

	m.RecordAdmissionRequests(resourceKind, resourceNamespace, resourceRequestOperation, pc.Log)

	return nil
}

func ProcessEngineResponses(pc *metrics.PromConfig, m *metrics.MetricsConfig, engineResponses []*response.EngineResponse, resourceRequestOperation metrics.ResourceRequestOperation) error {
	if len(engineResponses) == 0 {
		return nil
	}
	resourceNamespace, resourceKind := engineResponses[0].PolicyResponse.Resource.Namespace, engineResponses[0].PolicyResponse.Resource.Kind
	validateRulesCount, mutateRulesCount, generateRulesCount := 0, 0, 0
	for _, e := range engineResponses {
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
	}
	if validateRulesCount == 0 && mutateRulesCount == 0 && generateRulesCount == 0 {
		return nil
	}
	return registerAdmissionRequestsMetric(pc, m, resourceKind, resourceNamespace, resourceRequestOperation)
}
