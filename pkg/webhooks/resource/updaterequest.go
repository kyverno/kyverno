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
	"github.com/kyverno/kyverno/pkg/webhooks/resource/generation"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

// createUpdateRequests applies generate and mutateExisting policies, and creates update requests for background reconcile
func (h *handlers) createUpdateRequests(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, generatePolicies, mutatePolicies []kyvernov1.PolicyInterface, ts time.Time) {
	gh := generation.NewGenerationHandler(logger, h.client, h.kyvernoClient, h.nsLister, h.urLister, h.urGenerator, h.urUpdater, h.eventGen)
	go h.handleMutateExisting(logger, request, mutatePolicies, policyContext, ts)
	go gh.Handle(h.metricsConfig, request, generatePolicies, policyContext, ts)
}

func (h *handlers) handleMutateExisting(logger logr.Logger, request *admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface, policyContext *engine.PolicyContext, admissionRequestTimestamp time.Time) {
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
		logger.V(4).Info("update request for mutateExisting policy")

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
		go webhookutils.RegisterPolicyResultsMetricMutation(logger, h.metricsConfig, string(request.Operation), policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go webhookutils.RegisterPolicyExecutionDurationMetricMutate(logger, h.metricsConfig, string(request.Operation), policy, *engineResponse)
	}

	if failedResponse := applyUpdateRequest(request, kyvernov1beta1.Mutate, h.urGenerator, policyContext.AdmissionInfo, request.Operation, engineResponses...); failedResponse != nil {
		for _, failedUR := range failedResponse {
			err := fmt.Errorf("failed to create update request: %v", failedUR.err)
			events := event.NewBackgroundFailedEvent(err, failedUR.ur.Policy, "", event.GeneratePolicyController, &policyContext.NewResource)
			h.eventGen.Add(events...)
		}
	}
}
