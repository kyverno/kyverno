package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/generation"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

// handleBackgroundApplies applies generate and mutateExisting policies, and creates update requests for background reconcile
func (h *resourceHandlers) handleBackgroundApplies(ctx context.Context, logger logr.Logger, request admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, generatePolicies, mutatePolicies []kyvernov1.PolicyInterface, ts time.Time) {
	go h.handleMutateExisting(ctx, logger, request, mutatePolicies, policyContext, ts)
	h.handleGenerate(ctx, logger, request, generatePolicies, policyContext, ts)
}

func (h *resourceHandlers) handleMutateExisting(ctx context.Context, logger logr.Logger, request admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface, policyContext *engine.PolicyContext, admissionRequestTimestamp time.Time) {
	if request.Operation == admissionv1.Delete {
		policyContext = policyContext.WithNewResource(policyContext.OldResource())
	}

	var engineResponses []*engineapi.EngineResponse
	for _, policy := range policies {
		if !policy.GetSpec().IsMutateExisting() {
			continue
		}

		policyNew := skipBackgroundRequests(policy, logger, h.backgroundServiceAccountName, policyContext.AdmissionInfo().AdmissionUserInfo.Username)
		if policyNew == nil {
			continue
		}
		logger.V(4).Info("update request for mutateExisting policy")

		// skip rules that don't specify the DELETE operation in case the admission request is of type DELETE
		var skipped []string
		for _, rule := range autogen.ComputeRules(policy) {
			if request.Operation == admissionv1.Delete && !webhookutils.MatchDeleteOperation(rule) {
				skipped = append(skipped, rule.Name)
			}
		}

		var rules []engineapi.RuleResponse
		policyContext := policyContext.WithPolicy(policyNew)
		engineResponse := h.engine.ApplyBackgroundChecks(ctx, policyContext)

		for _, rule := range engineResponse.PolicyResponse.Rules {
			if rule.Status() == engineapi.RuleStatusPass && !datautils.SliceContains(skipped, rule.Name()) {
				rules = append(rules, rule)
			}
		}

		if len(rules) > 0 {
			engineResponse.PolicyResponse.Rules = rules
			engineResponses = append(engineResponses, &engineResponse)
		}
	}

	if failedResponse := applyUpdateRequest(ctx, request, kyvernov1beta1.Mutate, h.urGenerator, policyContext.AdmissionInfo(), request.Operation, engineResponses...); failedResponse != nil {
		for _, failedUR := range failedResponse {
			err := fmt.Errorf("failed to create update request: %v", failedUR.err)

			var policy kyvernov1.PolicyInterface
			for _, pol := range policies {
				if pol.GetName() != failedUR.ur.Policy {
					continue
				}
				policy = pol
			}
			resource := policyContext.NewResource()
			events := event.NewBackgroundFailedEvent(err, policy, "", event.GeneratePolicyController,
				kyvernov1.ResourceSpec{Kind: resource.GetKind(), Namespace: resource.GetNamespace(), Name: resource.GetName()})
			h.eventGen.Add(events...)
		}
	}
}

func (h *resourceHandlers) handleGenerate(ctx context.Context, logger logr.Logger, request admissionv1.AdmissionRequest, generatePolicies []kyvernov1.PolicyInterface, policyContext *engine.PolicyContext, ts time.Time) {
	gh := generation.NewGenerationHandler(logger, h.engine, h.client, h.kyvernoClient, h.nsLister, h.urLister, h.cpolLister, h.polLister, h.urGenerator, h.eventGen, h.metricsConfig, h.backgroundServiceAccountName)
	var policies []kyvernov1.PolicyInterface
	for _, p := range generatePolicies {
		new := skipBackgroundRequests(p, logger, h.backgroundServiceAccountName, policyContext.AdmissionInfo().AdmissionUserInfo.Username)
		if new != nil {
			policies = append(policies, new)
		}
	}
	go gh.Handle(ctx, request, policies, policyContext)
}
