package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/generation"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// handleBackgroundApplies applies generate and mutateExisting policies, and creates update requests for background reconcile
func (h *resourceHandlers) handleBackgroundApplies(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, generatePolicies, mutatePolicies []kyvernov1.PolicyInterface, ts time.Time, wg *wait.Group) {
	wg.Start(func() { h.handleMutateExisting(ctx, logger, request, mutatePolicies, ts) })
	wg.Start(func() { h.handleGenerate(ctx, logger, request, generatePolicies, ts) })
}

func (h *resourceHandlers) handleMutateExisting(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, policies []kyvernov1.PolicyInterface, admissionRequestTimestamp time.Time) {
	policyContext, err := h.buildPolicyContextFromAdmissionRequest(logger, request, policies)
	if err != nil {
		logger.Error(err, "failed to create policy context")
		return
	}

	if request.AdmissionRequest.Operation == admissionv1.Delete {
		policyContext = policyContext.WithNewResource(policyContext.OldResource())
	}

	var engineResponses []*engineapi.EngineResponse
	for _, policy := range policies {
		if !policy.GetSpec().HasMutateExisting() {
			continue
		}

		policyNew := skipBackgroundRequests(policy, logger, h.backgroundServiceAccountName, policyContext.AdmissionInfo().AdmissionUserInfo.Username)
		if policyNew == nil {
			continue
		}
		logger.V(4).Info("update request for mutateExisting policy")

		// skip rules that don't specify the DELETE operation in case the admission request is of type DELETE
		var skipped []string
		for _, rule := range autogen.Default.ComputeRules(policy, "") {
			if request.AdmissionRequest.Operation == admissionv1.Delete && !webhookutils.MatchDeleteOperation(rule) {
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

	if failedResponse := applyUpdateRequest(ctx, request.AdmissionRequest, kyvernov2.Mutate, h.urGenerator, policyContext.AdmissionInfo(), request.Operation, engineResponses...); failedResponse != nil {
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

func (h *resourceHandlers) handleGenerate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, generatePolicies []kyvernov1.PolicyInterface, ts time.Time) {
	policyContext, err := h.buildPolicyContextFromAdmissionRequest(logger, request, generatePolicies)
	if err != nil {
		logger.Error(err, "failed to create policy context")
		return
	}

	gh := generation.NewGenerationHandler(logger, h.engine, h.client, h.kyvernoClient, h.nsLister, h.urLister, h.cpolLister, h.polLister, h.urGenerator, h.eventGen, h.metricsConfig, h.backgroundServiceAccountName, h.reportsServiceAccountName)
	var policies []kyvernov1.PolicyInterface
	for _, p := range generatePolicies {
		new := skipBackgroundRequests(p, logger, h.backgroundServiceAccountName, policyContext.AdmissionInfo().AdmissionUserInfo.Username)
		if new != nil {
			policies = append(policies, new)
		}
	}
	gh.Handle(ctx, request.AdmissionRequest, policies, policyContext)
}
