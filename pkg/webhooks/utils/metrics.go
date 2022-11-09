package utils

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyExecutionDuration "github.com/kyverno/kyverno/pkg/metrics/policyexecutionduration"
	policyResults "github.com/kyverno/kyverno/pkg/metrics/policyresults"
)

type reporterFunc func(metrics.ResourceRequestOperation) error

func registerMetric(logger logr.Logger, m string, requestOperation string, r reporterFunc) {
	if op, err := metrics.ParseResourceRequestOperation(requestOperation); err != nil {
		logger.Error(err, fmt.Sprintf("error occurred while registering %s metrics", m))
	} else {
		if err := r(op); err != nil {
			logger.Error(err, fmt.Sprintf("error occurred while registering %s metrics", m))
		}
	}
}

// POLICY RESULTS

func RegisterPolicyResultsMetricMutation(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func RegisterPolicyResultsMetricValidation(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func RegisterPolicyResultsMetricGeneration(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

// POLICY EXECUTION

func RegisterPolicyExecutionDurationMetricMutate(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func RegisterPolicyExecutionDurationMetricValidate(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func RegisterPolicyExecutionDurationMetricGenerate(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}
