package generation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	generateutils "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	utils "github.com/kyverno/kyverno/pkg/utils/engine"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type GenerationHandler interface {
	Handle(context.Context, admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext)
}

func NewGenerationHandler(
	log logr.Logger,
	engine engineapi.Engine,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	nsLister corev1listers.NamespaceLister,
	urLister kyvernov2listers.UpdateRequestNamespaceLister,
	cpolLister kyvernov1listers.ClusterPolicyLister,
	polLister kyvernov1listers.PolicyLister,
	urGenerator webhookgenerate.Generator,
	eventGen event.Interface,
	metrics metrics.MetricsConfigManager,
	backgroundServiceAccountName string,
	reportsServiceAccountName string,
) GenerationHandler {
	return &generationHandler{
		log:                          log,
		engine:                       engine,
		client:                       client,
		kyvernoClient:                kyvernoClient,
		nsLister:                     nsLister,
		urLister:                     urLister,
		cpolLister:                   cpolLister,
		polLister:                    polLister,
		urGenerator:                  urGenerator,
		eventGen:                     eventGen,
		metrics:                      metrics,
		backgroundServiceAccountName: backgroundServiceAccountName,
		reportsServiceAccountName:    reportsServiceAccountName,
	}
}

type generationHandler struct {
	log                          logr.Logger
	engine                       engineapi.Engine
	client                       dclient.Interface
	kyvernoClient                versioned.Interface
	nsLister                     corev1listers.NamespaceLister
	urLister                     kyvernov2listers.UpdateRequestNamespaceLister
	cpolLister                   kyvernov1listers.ClusterPolicyLister
	polLister                    kyvernov1listers.PolicyLister
	urGenerator                  webhookgenerate.Generator
	eventGen                     event.Interface
	metrics                      metrics.MetricsConfigManager
	backgroundServiceAccountName string
	reportsServiceAccountName    string
}

func (h *generationHandler) Handle(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) {
	h.log.V(6).Info("handle admission request for generate")
	if len(policies) != 0 {
		h.handleTrigger(ctx, request, policies, policyContext)
	}

	if h.backgroundServiceAccountName == policyContext.AdmissionInfo().AdmissionUserInfo.Username {
		return
	}
	h.handleNonTrigger(ctx, policyContext)
}

func getAppliedRules(policy kyvernov1.PolicyInterface, applied []engineapi.RuleResponse) []kyvernov1.Rule {
	rules := []kyvernov1.Rule{}
	for _, rule := range policy.GetSpec().Rules {
		if !rule.HasGenerate() {
			continue
		}
		for _, applied := range applied {
			if applied.Name() == rule.Name && applied.RuleType() == engineapi.Generation {
				rules = append(rules, rule)
			}
		}
	}
	return rules
}

func (h *generationHandler) handleTrigger(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
) {
	h.log.V(4).Info("handle trigger resource operation for generate", "policies", len(policies))
	for _, policy := range policies {
		var appliedRules, failedRules []engineapi.RuleResponse
		policyContext := policyContext.WithPolicy(policy)
		if request.Kind.Kind != "Namespace" && request.Namespace != "" {
			policyContext = policyContext.WithNamespaceLabels(utils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log))
		}
		engineResponse := h.engine.ApplyBackgroundChecks(ctx, policyContext)
		for _, rule := range engineResponse.PolicyResponse.Rules {
			if rule.Status() == engineapi.RuleStatusPass {
				appliedRules = append(appliedRules, rule)
			} else if rule.Status() == engineapi.RuleStatusFail {
				failedRules = append(failedRules, rule)
			}
		}

		h.applyGeneration(ctx, request, policy, appliedRules, policyContext)
		h.syncTriggerAction(ctx, request, policy, failedRules, policyContext)
	}
}

func (h *generationHandler) handleNonTrigger(
	ctx context.Context,
	policyContext *engine.PolicyContext,
) {
	resource := policyContext.OldResource()
	labels := resource.GetLabels()
	if _, ok := labels[common.GenerateTypeCloneSourceLabel]; ok || labels[common.GeneratePolicyLabel] != "" {
		h.log.V(4).Info("handle non-trigger resource operation for generate")
		if err := h.processRequest(ctx, policyContext); err != nil {
			h.log.Error(err, "failed to create the UR on non-trigger admission request")
		}
	}
}

func (h *generationHandler) applyGeneration(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
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
		UID:        trigger.GetUID(),
	}

	rules := getAppliedRules(policy, appliedRules)
	h.log.V(4).Info("creating the UR to generate downstream on trigger's operation", "operation", request.Operation, "policy", pKey)
	urSpec := buildURSpecNew(kyvernov2.Generate, pKey, rules, triggerSpec, false)
	urSpec.Context = buildURContext(request, policyContext)
	if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
		h.log.Error(err, "failed to create the UR to create downstream on trigger's operation", "operation", request.Operation, "policy", pKey)
		e := event.NewFailedEvent(err, pKey, "", event.GeneratePolicyController,
			kyvernov1.ResourceSpec{Kind: policy.GetKind(), Namespace: policy.GetNamespace(), Name: policy.GetName()})
		h.eventGen.Add(e)
	}
}

// handleFailedRules sync changes of the trigger to the downstream
// it can be 1. trigger deletion; 2. trigger no longer matches, when a rule fails or is skipped
func (h *generationHandler) syncTriggerAction(
	ctx context.Context,
	request admissionv1.AdmissionRequest,
	policy kyvernov1.PolicyInterface,
	failedRules []engineapi.RuleResponse,
	policyContext *engine.PolicyContext,
) {
	if len(failedRules) == 0 {
		return
	}

	pKey := common.PolicyKey(policy.GetNamespace(), policy.GetName())
	trigger := policyContext.OldResource()
	triggerSpec := kyvernov1.ResourceSpec{
		APIVersion: trigger.GetAPIVersion(),
		Kind:       trigger.GetKind(),
		Namespace:  trigger.GetNamespace(),
		Name:       trigger.GetName(),
		UID:        trigger.GetUID(),
	}

	rules := getAppliedRules(policy, failedRules)
	urSpec := kyvernov2.UpdateRequestSpec{
		Type:        kyvernov2.Generate,
		Policy:      pKey,
		RuleContext: make([]kyvernov2.RuleContext, 0),
		Context:     buildURContext(request, policyContext),
	}
	for _, rule := range rules {
		// fire generation on trigger deletion
		if (request.Operation == admissionv1.Delete) && webhookutils.MatchDeleteOperation(rule) {
			h.log.V(4).Info("creating the UR to generate downstream on trigger's deletion", "operation", request.Operation, "rule", rule.Name, "trigger", triggerSpec.String())
			ruleCtx := buildRuleContext(rule, triggerSpec, false)
			urSpec.RuleContext = append(urSpec.RuleContext, ruleCtx)
			continue
		}

		// delete downstream on trigger deletion
		if rule.Generation.Synchronize {
			h.log.V(4).Info("creating the UR to delete downstream on trigger's event", "operation", request.Operation, "rule", rule.Name, "trigger", triggerSpec.String())
			ruleCtx := buildRuleContext(rule, triggerSpec, true)
			urSpec.RuleContext = append(urSpec.RuleContext, ruleCtx)
		}
	}

	if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
		h.log.Error(err, "failed to create the UR on trigger's event", "operation", request.Operation, "policy", pKey)
		e := event.NewFailedEvent(err, pKey, "", event.GeneratePolicyController,
			kyvernov1.ResourceSpec{Kind: policy.GetKind(), Namespace: policy.GetNamespace(), Name: policy.GetName()})
		h.eventGen.Add(e)
	}
}

// processRequest determine if it needs to re-apply the generate rule to the source or the target changes
func (h *generationHandler) processRequest(ctx context.Context, policyContext *engine.PolicyContext) (err error) {
	var policy kyvernov1.PolicyInterface
	var labelsList []map[string]string
	var deleteDownstream bool

	new := policyContext.NewResource()
	old := policyContext.OldResource()
	labels := old.GetLabels()
	managedBy := labels[kyverno.LabelAppManagedBy] == kyverno.ValueKyvernoApp

	// clone source changes
	if !managedBy {
		if new.Object == nil {
			// clone source deletion
			deleteDownstream = true
		}
		// fetch targets that have the source name label
		targetSelector := map[string]string{
			common.GenerateSourceGroupLabel:   old.GroupVersionKind().Group,
			common.GenerateSourceVersionLabel: old.GroupVersionKind().Version,
			common.GenerateSourceKindLabel:    old.GetKind(),
			common.GenerateSourceNSLabel:      old.GetNamespace(),
			common.GenerateSourceNameLabel:    old.GetName(),
		}
		targets, err := common.FindDownstream(h.client, old.GetAPIVersion(), old.GetKind(), targetSelector)
		if err != nil {
			return fmt.Errorf("failed to list targets resources: %v", err)
		}

		for i := range targets.Items {
			l := targets.Items[i].GetLabels()
			labelsList = append(labelsList, l)
		}

		// fetch targets that have the source UID label
		targetSelector = map[string]string{
			common.GenerateSourceGroupLabel:   old.GroupVersionKind().Group,
			common.GenerateSourceVersionLabel: old.GroupVersionKind().Version,
			common.GenerateSourceKindLabel:    old.GetKind(),
			common.GenerateSourceNSLabel:      old.GetNamespace(),
			common.GenerateSourceUIDLabel:     string(old.GetUID()),
		}
		targets, err = common.FindDownstream(h.client, old.GetAPIVersion(), old.GetKind(), targetSelector)
		if err != nil {
			return fmt.Errorf("failed to list targets resources: %v", err)
		}

		for i := range targets.Items {
			l := targets.Items[i].GetLabels()
			labelsList = append(labelsList, l)
		}
	} else {
		labelsList = append(labelsList, labels)
	}

	for _, labels := range labelsList {
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
		urSpec := kyvernov2.UpdateRequestSpec{
			Type:        kyvernov2.Generate,
			Policy:      pKey,
			RuleContext: make([]kyvernov2.RuleContext, 0),
		}

		for _, rule := range policy.GetSpec().Rules {
			if rule.Name == pRuleName && rule.Generation.Synchronize {
				gvk, subresource := policyContext.ResourceKind()
				if err := engineutils.MatchesResourceDescription(
					old,
					rule,
					policyContext.AdmissionInfo(),
					policyContext.NamespaceLabels(),
					policy.GetNamespace(),
					gvk,
					subresource,
					policyContext.Operation(),
				); err == nil {
					h.log.V(4).Info("skip creating UR as the admission resource is both the source and the trigger")
					continue
				}

				ruleCtx := buildRuleContext(rule, generateutils.TriggerFromLabels(labels), deleteDownstream)
				urSpec.RuleContext = append(urSpec.RuleContext, ruleCtx)
			}
		}
		if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
			e := event.NewBackgroundFailedEvent(err, policy, "", event.GeneratePolicyController,
				kyvernov1.ResourceSpec{Kind: new.GetKind(), Namespace: new.GetNamespace(), Name: new.GetName()})
			h.eventGen.Add(e...)
			return err
		}
	}
	return nil
}
