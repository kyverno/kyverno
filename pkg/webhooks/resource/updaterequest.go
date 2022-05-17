package resource

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	admissionv1 "k8s.io/api/admission/v1"
)

// createUpdateRequests applies generate and mutateExisting policies, and creates update requests for background reconcile
func (h *handlers) createUpdateRequests(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, generatePolicies, mutatePolicies []kyvernov1.PolicyInterface, ts int64) {
	admissionReviewCompletionLatencyChannel := make(chan int64, 1)
	generateEngineResponsesSenderForAdmissionReviewDurationMetric := make(chan []*response.EngineResponse, 1)
	generateEngineResponsesSenderForAdmissionRequestsCountMetric := make(chan []*response.EngineResponse, 1)

	go h.handleMutateExisting(logger, request, mutatePolicies, policyContext, ts)
	go h.handleGenerate(logger, request, generatePolicies, policyContext, ts, &admissionReviewCompletionLatencyChannel, &generateEngineResponsesSenderForAdmissionReviewDurationMetric, &generateEngineResponsesSenderForAdmissionRequestsCountMetric)

	go h.registerAdmissionReviewDurationMetricGenerate(logger, string(request.Operation), &admissionReviewCompletionLatencyChannel, &generateEngineResponsesSenderForAdmissionReviewDurationMetric)
	go h.registerAdmissionRequestsMetricGenerate(logger, string(request.Operation), &generateEngineResponsesSenderForAdmissionRequestsCountMetric)
}

func (h *handlers) handleMutateExisting(logger logr.Logger, request *admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface, policyContext *engine.PolicyContext, admissionRequestTimestamp int64) {
	logger.V(4).Info("update request")

	if request.Operation == admissionv1.Delete {
		policyContext.NewResource = policyContext.OldResource
	}

	if request.Operation == admissionv1.Update && policyContext.NewResource.GetDeletionTimestamp() != nil {
		logger.V(4).Info("skip creating UR for the trigger resource that is in termination")
		return
	}

	var engineResponses []*response.EngineResponse
	for _, policy := range policies {
		if !policy.GetSpec().IsMutateExisting() {
			continue
		}

		var rules []response.RuleResponse
		policyContext.Policy = policy
		engineResponse := engine.ApplyBackgroundChecks(policyContext)

		for _, rule := range engineResponse.PolicyResponse.Rules {
			if rule.Status == response.RuleStatusPass {
				rules = append(rules, rule)
			}
		}

		if len(rules) > 0 {
			engineResponse.PolicyResponse.Rules = rules
			engineResponses = append(engineResponses, engineResponse)
		}

		// registering the kyverno_policy_results_total metric concurrently
		go h.registerPolicyResultsMetricMutation(logger, string(request.Operation), policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go h.registerPolicyExecutionDurationMetricMutate(logger, string(request.Operation), policy, *engineResponse)
	}

	if failedResponse := applyUpdateRequest(request, kyvernov1beta1.Mutate, h.urGenerator, policyContext.AdmissionInfo, request.Operation, engineResponses...); failedResponse != nil {
		for _, failedUR := range failedResponse {
			err := fmt.Errorf("failed to create update request: %v", failedUR.err)
			events := event.NewBackgroundFailedEvent(err, failedUR.ur.Policy, "", event.GeneratePolicyController, &policyContext.NewResource)
			h.eventGen.Add(events...)
		}
	}

	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	go h.registerAdmissionReviewDurationMetricMutate(logger, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
	go h.registerAdmissionRequestsMetricMutate(logger, string(request.Operation), engineResponses)
}
