package generation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	generateutils "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type GenerationHandler interface {
	HandleNew(context.Context, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext)
}

func NewGenerationHandler(
	log logr.Logger,
	engine engineapi.Engine,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	nsLister corev1listers.NamespaceLister,
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister,
	cpolLister kyvernov1listers.ClusterPolicyLister,
	polLister kyvernov1listers.PolicyLister,
	urGenerator webhookgenerate.Generator,
	urUpdater webhookutils.UpdateRequestUpdater,
	eventGen event.Interface,
	metrics metrics.MetricsConfigManager,
) GenerationHandler {
	return &generationHandler{
		log:           log,
		engine:        engine,
		client:        client,
		kyvernoClient: kyvernoClient,
		nsLister:      nsLister,
		urLister:      urLister,
		cpolLister:    cpolLister,
		polLister:     polLister,
		urGenerator:   urGenerator,
		urUpdater:     urUpdater,
		eventGen:      eventGen,
		metrics:       metrics,
	}
}

type generationHandler struct {
	log           logr.Logger
	engine        engineapi.Engine
	client        dclient.Interface
	kyvernoClient versioned.Interface
	nsLister      corev1listers.NamespaceLister
	urLister      kyvernov1beta1listers.UpdateRequestNamespaceLister
	cpolLister    kyvernov1listers.ClusterPolicyLister
	polLister     kyvernov1listers.PolicyLister
	urGenerator   webhookgenerate.Generator
	urUpdater     webhookutils.UpdateRequestUpdater
	eventGen      event.Interface
	metrics       metrics.MetricsConfigManager
}

func (h *generationHandler) HandleNew(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) {
	h.log.V(6).Info("handle admission request for generate")
	if len(policies) != 0 {
		h.handleTrigger(ctx, request, policies, policyContext)
	}

	h.handleNonTrigger(ctx, policyContext, request)
}

func (h *generationHandler) handleTrigger(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) {
	h.log.V(4).Info("handle trigger resource operation for generate")
	var engineResponses []*engineapi.EngineResponse
	for _, policy := range policies {
		var appliedRules []engineapi.RuleResponse
		policyContext := policyContext.WithPolicy(policy)
		if request.Kind.Kind != "Namespace" && request.Namespace != "" {
			policyContext = policyContext.WithNamespaceLabels(engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log))
		}
		engineResponse := h.engine.ApplyBackgroundChecks(ctx, policyContext)
		for _, rule := range engineResponse.PolicyResponse.Rules {
			if rule.Status == engineapi.RuleStatusPass {
				appliedRules = append(appliedRules, rule)
			}
		}

		if len(appliedRules) > 0 {
			engineResponse.PolicyResponse.Rules = appliedRules
			// some generate rules do apply to the resource
			engineResponses = append(engineResponses, engineResponse)
		}

		// registering the kyverno_policy_results_total metric concurrently
		go webhookutils.RegisterPolicyResultsMetricGeneration(ctx, h.log, h.metrics, string(request.Operation), policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go webhookutils.RegisterPolicyExecutionDurationMetricGenerate(ctx, h.log, h.metrics, string(request.Operation), policy, *engineResponse)
	}

	if failedResponse := applyUpdateRequest(ctx, request, kyvernov1beta1.Generate, h.urGenerator, policyContext.AdmissionInfo(), request.Operation, engineResponses...); failedResponse != nil {
		// report failure event
		for _, failedUR := range failedResponse {
			err := fmt.Errorf("failed to create Update Request: %v", failedUR.err)
			newResource := policyContext.NewResource()
			e := event.NewBackgroundFailedEvent(err, failedUR.ur.Policy, "", event.GeneratePolicyController, &newResource)
			h.eventGen.Add(e...)
		}
	}
}

func (h *generationHandler) handleNonTrigger(
	ctx context.Context,
	policyContext *engine.PolicyContext,
	request *admissionv1.AdmissionRequest,
) {
	resource := policyContext.OldResource()
	labels := resource.GetLabels()
	if labels[common.GeneratePolicyLabel] != "" {
		h.log.V(4).Info("handle non-trigger resource operation for generate")
		if err := h.createUR(ctx, policyContext, request); err != nil {
			h.log.Error(err, "failed to create the UR on non-trigger admission request")
		}
	}
}

func (h *generationHandler) createUR(ctx context.Context, policyContext *engine.PolicyContext, request *admissionv1.AdmissionRequest) (err error) {
	var policy kyvernov1.PolicyInterface
	new := policyContext.NewResource()
	labels := new.GetLabels()
	old := policyContext.OldResource()
	oldLabels := old.GetLabels()
	if !compareLabels(labels, oldLabels) {
		return fmt.Errorf("labels have been changed, new: %v, old: %v", labels, oldLabels)
	}

	deleteDownstream := false
	if reflect.DeepEqual(new, unstructured.Unstructured{}) {
		deleteDownstream = true
		labels = oldLabels
	}
	pName := labels[common.GeneratePolicyLabel]
	pNamespace := labels[common.GeneratePolicyNamespaceLabel]
	pRuleName := labels[common.GenerateRuleLabel]

	if pNamespace != "" {
		policy, err = h.polLister.Policies(pNamespace).Get(pName)
	} else {
		policy, err = h.cpolLister.Get(pName)
	}

	if err != nil {
		return err
	}

	pKey := common.PolicyKey(pNamespace, pName)
	for _, rule := range policy.GetSpec().Rules {
		if rule.Name == pRuleName && rule.Generation.Synchronize {
			ur := kyvernov1beta1.UpdateRequestSpec{
				Type:     kyvernov1beta1.Generate,
				Policy:   pKey,
				Rule:     rule.Name,
				Resource: generateutils.TriggerFromLabels(labels),
			}
			ur.DeleteDownstream = deleteDownstream
			if err := h.urGenerator.Apply(ctx, ur, admissionv1.Update); err != nil {
				e := event.NewBackgroundFailedEvent(err, pKey, pRuleName, event.GeneratePolicyController, &new)
				h.eventGen.Add(e...)
				return err
			}
		}
	}
	return nil
}

func compareLabels(new, old map[string]string) bool {
	if new == nil {
		return true
	}
	if new[common.GeneratePolicyLabel] != old[common.GeneratePolicyLabel] ||
		new[common.GeneratePolicyNamespaceLabel] != old[common.GeneratePolicyNamespaceLabel] ||
		new[common.GenerateRuleLabel] != old[common.GenerateRuleLabel] ||
		new[common.GenerateTriggerNameLabel] != old[common.GenerateTriggerNameLabel] ||
		new[common.GenerateTriggerNSLabel] != old[common.GenerateTriggerNSLabel] ||
		new[common.GenerateTriggerKindLabel] != old[common.GenerateTriggerKindLabel] {
		return false
	}
	return true
}
