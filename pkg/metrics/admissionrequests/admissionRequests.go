package admissionrequests

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
)

func registerAdmissionRequestsMetric(
	m *metrics.MetricsConfig,
	resourceKind, resourceNamespace string,
	resourceRequestOperation metrics.ResourceRequestOperation,
) error {
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (resourceNamespace != "" && resourceNamespace != "-") && utils.ContainsString(excludeNamespaces, resourceNamespace) {
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", resourceNamespace, excludeNamespaces))
		return nil
	}
	if (resourceNamespace != "" && resourceNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, resourceNamespace) {
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_admission_requests_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", resourceNamespace, includeNamespaces))
		return nil
	}

	m.RecordAdmissionRequests(resourceKind, resourceNamespace, resourceRequestOperation)

	return nil
}

func ProcessEngineResponses(m *metrics.MetricsConfig, engineResponses []*response.EngineResponse, resourceRequestOperation metrics.ResourceRequestOperation) error {
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
	return registerAdmissionRequestsMetric(m, resourceKind, resourceNamespace, resourceRequestOperation)
}
