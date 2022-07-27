package resource

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
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

// ADMISSION REVIEW

func (h *handlers) registerAdmissionReviewDurationMetricMutate(logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	registerMetric(logger, "kyverno_admission_review_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionReviewDuration.ProcessEngineResponses(h.metricsConfig, engineResponses, admissionReviewLatencyDuration, op)
	})
}

func (h *handlers) registerAdmissionReviewDurationMetricGenerate(logger logr.Logger, requestOperation string, latencyReceiver *chan int64, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*latencyReceiver)
	defer close(*engineResponsesReceiver)
	registerMetric(logger, "kyverno_admission_review_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionReviewDuration.ProcessEngineResponses(h.metricsConfig, <-(*engineResponsesReceiver), <-(*latencyReceiver), op)
	})
}

func registerAdmissionReviewDurationMetricValidate(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	registerMetric(logger, "kyverno_admission_review_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionReviewDuration.ProcessEngineResponses(metricsConfig, engineResponses, admissionReviewLatencyDuration, op)
	})
}

// ADMISSION REQUEST

func (h *handlers) registerAdmissionRequestsMetricMutate(logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse) {
	registerMetric(logger, "kyverno_admission_requests_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionRequests.ProcessEngineResponses(h.metricsConfig, engineResponses, op)
	})
}

func (h *handlers) registerAdmissionRequestsMetricGenerate(logger logr.Logger, requestOperation string, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*engineResponsesReceiver)
	registerMetric(logger, "kyverno_admission_requests_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionRequests.ProcessEngineResponses(h.metricsConfig, <-(*engineResponsesReceiver), op)
	})
}

func registerAdmissionRequestsMetricValidate(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, engineResponses []*response.EngineResponse) {
	registerMetric(logger, "kyverno_admission_requests_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionRequests.ProcessEngineResponses(metricsConfig, engineResponses, op)
	})
}

// POLICY RESULTS

func (h *handlers) registerPolicyResultsMetricMutation(logger logr.Logger, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(h.metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func registerPolicyResultsMetricValidation(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func (h *handlers) registerPolicyResultsMetricGeneration(logger logr.Logger, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(h.metricsConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

// POLICY EXECUTION

func (h *handlers) registerPolicyExecutionDurationMetricMutate(logger logr.Logger, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(h.metricsConfig, policy, engineResponse, metrics.AdmissionRequest, "", op)
	})
}

func registerPolicyExecutionDurationMetricValidate(logger logr.Logger, metricsConfig *metrics.MetricsConfig, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(metricsConfig, policy, engineResponse, metrics.AdmissionRequest, "", op)
	})
}

func (h *handlers) registerPolicyExecutionDurationMetricGenerate(logger logr.Logger, requestOperation string, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(h.metricsConfig, policy, engineResponse, metrics.AdmissionRequest, "", op)
	})
}
