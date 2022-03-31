package webhooks

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
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
