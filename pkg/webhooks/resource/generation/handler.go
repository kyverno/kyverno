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
	Handle(context.Context, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext)
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

func (h *generationHandler) Handle(
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

func getAppliedRules(policy kyvernov1.PolicyInterface, applied []engineapi.RuleResponse) []kyvernov1.Rule {
	rules := []kyvernov1.Rule{}
	for _, rule := range policy.GetSpec().Rules {
		if !rule.HasGenerate() {
			continue
		}
		for _, applied := range applied {
			if applied.Name == rule.Name && applied.Type == engineapi.Generation {
				rules = append(rules, rule)
			}
		}
	}
	return rules
}

func (h *generationHandler) handleTrigger(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) {
	h.log.V(4).Info("handle trigger resource operation for generate", "policies", len(policies))
	for _, policy := range policies {
		var appliedRules, failedRules []engineapi.RuleResponse
		policyContext := policyContext.WithPolicy(policy)
		if request.Kind.Kind != "Namespace" && request.Namespace != "" {
			policyContext = policyContext.WithNamespaceLabels(engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log))
		}
		engineResponse := h.engine.ApplyBackgroundChecks(ctx, policyContext)
		for _, rule := range engineResponse.PolicyResponse.Rules {
			if rule.Status == engineapi.RuleStatusPass {
				appliedRules = append(appliedRules, rule)
			} else if rule.Status == engineapi.RuleStatusFail {
				failedRules = append(failedRules, rule)
			}
		}

		h.applyGeneration(ctx, request, policy, appliedRules, policyContext)
		h.syncTriggerAction(ctx, request, policy, failedRules, policyContext)

		go webhookutils.RegisterPolicyResultsMetricGeneration(ctx, h.log, h.metrics, string(request.Operation), policy, *engineResponse)
		go webhookutils.RegisterPolicyExecutionDurationMetricGenerate(ctx, h.log, h.metrics, string(request.Operation), policy, *engineResponse)
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

func (h *generationHandler) applyGeneration(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policy kyvernov1.PolicyInterface,
	appliedRules []engineapi.RuleResponse,
	policyContext *engine.PolicyContext,
) {
	if len(appliedRules) == 0 {
		return
	}

	pKey := common.PolicyKey(policy.GetNamespace(), policy.GetName())
	trigger := policyContext.NewResource()
	triggerSpec := kyvernov1.ResourceSpec{
		APIVersion: trigger.GetAPIVersion(),
		Kind:       trigger.GetKind(),
		Namespace:  trigger.GetNamespace(),
		Name:       trigger.GetName(),
	}

	rules := getAppliedRules(policy, appliedRules)
	for _, rule := range rules {
		h.log.V(4).Info("creating the UR to generate downstream on trigger's operation", "operation", request.Operation, "rule", rule.Name)
		urSpec := buildURSpec(kyvernov1beta1.Generate, pKey, rule.Name, triggerSpec, false)
		urSpec.Context = buildURContext(request, policyContext)
		if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
			h.log.Error(err, "failed to create the UR to create downstream on trigger's operation", "operation", request.Operation, "rule", rule.Name)
			e := event.NewFailedEvent(err, pKey, rule.Name, event.GeneratePolicyController,
				kyvernov1.ResourceSpec{Kind: policy.GetKind(), Namespace: policy.GetNamespace(), Name: policy.GetName()})
			h.eventGen.Add(e)
		}
	}
}

// handleFailedRules sync changes of the trigger to the downstream
// it can be 1. trigger deletion; 2. trigger no longer matches, when a rule fails
func (h *generationHandler) syncTriggerAction(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policy kyvernov1.PolicyInterface,
	failedRules []engineapi.RuleResponse,
	policyContext *engine.PolicyContext,
) {
	if len(failedRules) == 0 {
		return
	}

	pKey := common.PolicyKey(policy.GetNamespace(), policy.GetName())
	trigger := policyContext.OldResource()
	urSpec := kyvernov1.ResourceSpec{
		APIVersion: trigger.GetAPIVersion(),
		Kind:       trigger.GetKind(),
		Namespace:  trigger.GetNamespace(),
		Name:       trigger.GetName(),
	}

	rules := getAppliedRules(policy, failedRules)
	for _, rule := range rules {
		// fire generation on trigger deletion
		if (request.Operation == admissionv1.Delete) && precondition(rule, kyvernov1.Condition{
			RawKey:   kyvernov1.ToJSON("request.operation"),
			Operator: "Equals",
			RawValue: kyvernov1.ToJSON("DELETE"),
		}) {
			h.log.V(4).Info("creating the UR to generate downstream on trigger's deletion", "operation", request.Operation, "rule", rule.Name)
			ur := buildURSpec(kyvernov1beta1.Generate, pKey, rule.Name, urSpec, false)
			ur.Context = buildURContext(request, policyContext)
			if err := h.urGenerator.Apply(ctx, ur); err != nil {
				h.log.Error(err, "failed to create the UR to generate downstream on trigger's deletion", "operation", request.Operation, "rule", rule.Name)
				e := event.NewFailedEvent(err, pKey, rule.Name, event.GeneratePolicyController,
					kyvernov1.ResourceSpec{Kind: policy.GetKind(), Namespace: policy.GetNamespace(), Name: policy.GetName()})
				h.eventGen.Add(e)
			}
			continue
		}

		// delete downstream on trigger deletion
		if rule.Generation.Synchronize {
			h.log.V(4).Info("creating the UR to delete downstream on trigger's event", "operation", request.Operation, "rule", rule.Name)
			ur := buildURSpec(kyvernov1beta1.Generate, pKey, rule.Name, urSpec, true)
			ur.Context = buildURContext(request, policyContext)
			if err := h.urGenerator.Apply(ctx, ur); err != nil {
				h.log.Error(err, "failed to create the UR to delete downstream on trigger's event", "operation", request.Operation, "rule", rule.Name)
				e := event.NewFailedEvent(err, pKey, rule.Name, event.GeneratePolicyController,
					kyvernov1.ResourceSpec{Kind: policy.GetKind(), Namespace: policy.GetNamespace(), Name: policy.GetName()})
				h.eventGen.Add(e)
			}
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

	managedBy := oldLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp
	deleteDownstream := false
	if reflect.DeepEqual(new, unstructured.Unstructured{}) {
		labels = oldLabels
		if !managedBy {
			deleteDownstream = true
		}
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
			ur := buildURSpec(kyvernov1beta1.Generate, pKey, rule.Name, generateutils.TriggerFromLabels(labels), deleteDownstream)
			if err := h.urGenerator.Apply(ctx, ur); err != nil {
				e := event.NewBackgroundFailedEvent(err, pKey, pRuleName, event.GeneratePolicyController, &new)
				h.eventGen.Add(e...)
				return err
			}
		}
	}
	return nil
}
