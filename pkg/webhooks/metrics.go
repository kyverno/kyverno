package webhooks

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
	policyExecutionDuration "github.com/kyverno/kyverno/pkg/metrics/policyexecutionduration"
	policyResults "github.com/kyverno/kyverno/pkg/metrics/policyresults"
)

func registerAdmissionReviewDurationMetricMutate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
	if err := admissionReviewDuration.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, admissionReviewLatencyDuration, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
}

func registerAdmissionRequestsMetricMutate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, engineResponses []*response.EngineResponse) {
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
	if err := admissionRequests.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
}

func registerAdmissionReviewDurationMetricGenerate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, latencyReceiver *chan int64, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*latencyReceiver)
	defer close(*engineResponsesReceiver)
	engineResponses := <-(*engineResponsesReceiver)
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
	// this goroutine will keep on waiting here till it doesn't receive the admission review latency int64 from the other goroutine i.e. ws.HandleGenerate
	admissionReviewLatencyDuration := <-(*latencyReceiver)
	if err := admissionReviewDuration.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, admissionReviewLatencyDuration, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
}

func registerAdmissionRequestsMetricGenerate(logger logr.Logger, promConfig metrics.PromConfig, requestOperation string, engineResponsesReceiver *chan []*response.EngineResponse) {
	defer close(*engineResponsesReceiver)
	engineResponses := <-(*engineResponsesReceiver)
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
	if err := admissionRequests.ParsePromConfig(promConfig).ProcessEngineResponses(engineResponses, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
}

func registerPolicyResultsMetricValidation(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, policy v1.PolicyInterface, engineResponse response.EngineResponse) {
	resourceRequestOperationPromAlias, err := policyResults.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.GetName())
	}
	if err := policyResults.ParsePromConfig(*promConfig).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.GetName())
	}
}

func registerPolicyExecutionDurationMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, policy v1.PolicyInterface, engineResponse response.EngineResponse) {
	resourceRequestOperationPromAlias, err := policyExecutionDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.GetName())
	}
	if err := policyExecutionDuration.ParsePromConfig(*promConfig).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, "", resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.GetName())
	}
}

func registerAdmissionReviewDurationMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
	if err := admissionReviewDuration.ParsePromConfig(*promConfig).ProcessEngineResponses(engineResponses, admissionReviewLatencyDuration, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
}

func registerAdmissionRequestsMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse) {
	resourceRequestOperationPromAlias, err := admissionRequests.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
	if err := admissionRequests.ParsePromConfig(*promConfig).ProcessEngineResponses(engineResponses, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
}

func (ws *WebhookServer) registerPolicyResultsMetricMutation(logger logr.Logger, resourceRequestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	resourceRequestOperationPromAlias, err := policyResults.ParseResourceRequestOperation(resourceRequestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.GetName())
	}
	if err := policyResults.ParsePromConfig(*ws.promConfig).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.GetName())
	}
}

func (ws *WebhookServer) registerPolicyExecutionDurationMetricMutate(logger logr.Logger, resourceRequestOperation string, policy kyverno.PolicyInterface, engineResponse response.EngineResponse) {
	resourceRequestOperationPromAlias, err := policyExecutionDuration.ParseResourceRequestOperation(resourceRequestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.GetName())
	}
	if err := policyExecutionDuration.ParsePromConfig(*ws.promConfig).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, "", resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.GetName())
	}
}
