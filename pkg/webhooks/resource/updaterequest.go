package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/generation"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

// handleBackgroundApplies applies generate and mutateExisting policies, and creates update requests for background reconcile
func (h *handlers) handleBackgroundApplies(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, generatePolicies, mutatePolicies []kyvernov1.PolicyInterface, ts time.Time) {
	go h.handleMutateExisting(ctx, logger, request, mutatePolicies, policyContext, ts)
	h.handleGenerate(ctx, logger, request, generatePolicies, policyContext, ts)
}

func (h *handlers) handleMutateExisting(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface, policyContext *engine.PolicyContext, admissionRequestTimestamp time.Time) {
	if request.Operation == admissionv1.Delete {
		policyContext = policyContext.WithNewResource(policyContext.OldResource())
	}

	resource := policyContext.NewResource()
	if request.Operation == admissionv1.Update && resource.GetDeletionTimestamp() != nil {
		logger.V(4).Info("skip creating UR for the trigger resource that is in termination")
		return
	}

	var engineResponses []*engineapi.EngineResponse
	for _, policy := range policies {
		if !policy.GetSpec().IsMutateExisting() {
			continue
		}
		logger.V(4).Info("update request for mutateExisting policy")

		var rules []engineapi.RuleResponse
		policyContext := policyContext.WithPolicy(policy)
		engineResponse := h.engine.ApplyBackgroundChecks(ctx, policyContext)

		for _, rule := range engineResponse.PolicyResponse.Rules {
			if rule.Status == engineapi.RuleStatusPass {
				rules = append(rules, rule)
			}
		}

		if len(rules) > 0 {
			engineResponse.PolicyResponse.Rules = rules
			engineResponses = append(engineResponses, engineResponse)
		}

		// registering the kyverno_policy_results_total metric concurrently
		go webhookutils.RegisterPolicyResultsMetricMutation(context.TODO(), logger, h.metricsConfig, string(request.Operation), policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go webhookutils.RegisterPolicyExecutionDurationMetricMutate(context.TODO(), logger, h.metricsConfig, string(request.Operation), policy, *engineResponse)
	}

	if failedResponse := applyUpdateRequest(ctx, request, kyvernov1beta1.Mutate, h.urGenerator, policyContext.AdmissionInfo(), request.Operation, engineResponses...); failedResponse != nil {
		for _, failedUR := range failedResponse {
			err := fmt.Errorf("failed to create update request: %v", failedUR.err)
			resource := policyContext.NewResource()
			events := event.NewBackgroundFailedEvent(err, failedUR.ur.Policy, "", event.GeneratePolicyController, &resource)
			h.eventGen.Add(events...)
		}
	}
}

func (h *handlers) handleGenerate(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, generatePolicies []kyvernov1.PolicyInterface, policyContext *engine.PolicyContext, ts time.Time) {
	gh := generation.NewGenerationHandler(logger, h.engine, h.client, h.kyvernoClient, h.nsLister, h.urLister, h.cpolLister, h.polLister, h.urGenerator, h.urUpdater, h.eventGen, h.metricsConfig)
	// go gh.Handle(ctx, request, generatePolicies, policyContext, ts)
	go gh.HandleNew(ctx, request, generatePolicies, policyContext)
}
