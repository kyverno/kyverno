package webhooks

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	admissionv1 "k8s.io/api/admission/v1"
)

// createUpdateRequests applies generate and mutateExisting policies, and creates update requests for background reconcile
func (ws *WebhookServer) createUpdateRequests(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, generatePolicies, mutatePolicies []kyverno.PolicyInterface, ts int64, logger logr.Logger) {
	admissionReviewCompletionLatencyChannel := make(chan int64, 1)
	generateEngineResponsesSenderForAdmissionReviewDurationMetric := make(chan []*response.EngineResponse, 1)
	generateEngineResponsesSenderForAdmissionRequestsCountMetric := make(chan []*response.EngineResponse, 1)

	go ws.handleMutateExisting(request, mutatePolicies, policyContext, ts)

	go ws.handleGenerate(request, generatePolicies, policyContext, ts, &admissionReviewCompletionLatencyChannel, &generateEngineResponsesSenderForAdmissionReviewDurationMetric, &generateEngineResponsesSenderForAdmissionRequestsCountMetric)
	go ws.registerAdmissionReviewDurationMetricGenerate(logger, string(request.Operation), &admissionReviewCompletionLatencyChannel, &generateEngineResponsesSenderForAdmissionReviewDurationMetric)
	go ws.registerAdmissionRequestsMetricGenerate(logger, string(request.Operation), &generateEngineResponsesSenderForAdmissionRequestsCountMetric)
}

func (ws *WebhookServer) handleMutateExisting(request *admissionv1.AdmissionRequest, policies []kyverno.PolicyInterface, policyContext *engine.PolicyContext, admissionRequestTimestamp int64) {

	logger := ws.log.WithValues("action", "mutateExisting", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	logger.V(6).Info("update request")

	var engineResponses []*response.EngineResponse
	for _, policy := range policies {
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
		go ws.registerPolicyResultsMetricMutation(logger, string(request.Operation), policy, *engineResponse)

		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go ws.registerPolicyExecutionDurationMetricMutate(logger, string(request.Operation), policy, *engineResponse)
	}

	if failedResponse := applyGenerateRequest(request, ws.grGenerator, policyContext.AdmissionInfo, request.Operation, engineResponses...); failedResponse != nil {
		for _, failedGR := range failedResponse {
			events := failedEvents(fmt.Errorf("failed to create Generate Request: %v", failedGR.err), failedGR.gr, policyContext.NewResource)
			ws.eventGen.Add(events...)
		}
	}

	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	go ws.registerAdmissionReviewDurationMetricMutate(logger, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
	go ws.registerAdmissionRequestsMetricMutate(logger, string(request.Operation), engineResponses)
}
