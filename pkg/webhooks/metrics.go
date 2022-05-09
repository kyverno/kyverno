package webhooks

import (
	"fmt"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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

func (ws *WebhookServer) registerAdmissionReviewDurationMetricMutate(logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	registerMetric(logger, "kyverno_admission_review_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionReviewDuration.ProcessEngineResponses(ws.promConfig, engineResponses, admissionReviewLatencyDuration, op)
	})
}

func (ws *WebhookServer) registerAdmissionReviewDurationMetricGenerate(logger logr.Logger, requestOperation string, latencyReceiver *chan int64, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*latencyReceiver)
	defer close(*engineResponsesReceiver)
	registerMetric(logger, "kyverno_admission_review_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionReviewDuration.ProcessEngineResponses(ws.promConfig, <-(*engineResponsesReceiver), <-(*latencyReceiver), op)
	})
}

func registerAdmissionReviewDurationMetricValidate(logger logr.Logger, promConfig *metrics.PromConfig, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	registerMetric(logger, "kyverno_admission_review_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionReviewDuration.ProcessEngineResponses(promConfig, engineResponses, admissionReviewLatencyDuration, op)
	})
}

// ADMISSION REQUEST

func (ws *WebhookServer) registerAdmissionRequestsMetricMutate(logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse) {
	registerMetric(logger, "kyverno_admission_requests_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionRequests.ProcessEngineResponses(ws.promConfig, engineResponses, op)
	})
}

func (ws *WebhookServer) registerAdmissionRequestsMetricGenerate(logger logr.Logger, requestOperation string, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*engineResponsesReceiver)
	registerMetric(logger, "kyverno_admission_requests_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionRequests.ProcessEngineResponses(ws.promConfig, <-(*engineResponsesReceiver), op)
	})
}

func registerAdmissionRequestsMetricValidate(logger logr.Logger, promConfig *metrics.PromConfig, requestOperation string, engineResponses []*response.EngineResponse) {
	registerMetric(logger, "kyverno_admission_requests_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return admissionRequests.ProcessEngineResponses(promConfig, engineResponses, op)
	})
}

// POLICY RESULTS

func (ws *WebhookServer) registerPolicyResultsMetricMutation(logger logr.Logger, requestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(ws.promConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func registerPolicyResultsMetricValidation(logger logr.Logger, promConfig *metrics.PromConfig, requestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(promConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

func (ws *WebhookServer) registerPolicyResultsMetricGeneration(logger logr.Logger, requestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_results_total", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyResults.ProcessEngineResponse(ws.promConfig, policy, engineResponse, metrics.AdmissionRequest, op)
	})
}

// POLICY EXECUTION

func (ws *WebhookServer) registerPolicyExecutionDurationMetricMutate(logger logr.Logger, requestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(ws.promConfig, policy, engineResponse, metrics.AdmissionRequest, "", op)
	})
}

func registerPolicyExecutionDurationMetricValidate(logger logr.Logger, promConfig *metrics.PromConfig, requestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(promConfig, policy, engineResponse, metrics.AdmissionRequest, "", op)
	})
}

func (ws *WebhookServer) registerPolicyExecutionDurationMetricGenerate(logger logr.Logger, requestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	registerMetric(logger, "kyverno_policy_execution_duration_seconds", requestOperation, func(op metrics.ResourceRequestOperation) error {
		return policyExecutionDuration.ProcessEngineResponse(ws.promConfig, policy, engineResponse, metrics.AdmissionRequest, "", op)
	})
}
