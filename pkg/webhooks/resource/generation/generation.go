package generation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/generate"
	gen "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type GenerationHandler interface {
	Handle(context.Context, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, time.Time)
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

// Handle handles admission-requests for policies with generate rules
func (h *generationHandler) Handle(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
) {
	h.log.V(6).Info("update request for generate policy")

	var engineResponses []*engineapi.EngineResponse
	if (request.Operation == admissionv1.Create || request.Operation == admissionv1.Update) && len(policies) != 0 {
		for _, policy := range policies {
			var rules []engineapi.RuleResponse
			policyContext := policyContext.WithPolicy(policy)
			if request.Kind.Kind != "Namespace" && request.Namespace != "" {
				policyContext = policyContext.WithNamespaceLabels(engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log))
			}
			engineResponse := h.engine.ApplyBackgroundChecks(ctx, policyContext)
			for _, rule := range engineResponse.PolicyResponse.Rules {
				if rule.Status != engineapi.RuleStatusPass {
					h.deleteGR(ctx, engineResponse)
					continue
				}
				rules = append(rules, rule)
			}

			if len(rules) > 0 {
				engineResponse.PolicyResponse.Rules = rules
				// some generate rules do apply to the resource
				engineResponses = append(engineResponses, engineResponse)
			}

			// registering the kyverno_policy_results_total metric concurrently
			go webhookutils.RegisterPolicyResultsMetricGeneration(context.TODO(), h.log, h.metrics, string(request.Operation), policy, *engineResponse)
			// registering the kyverno_policy_execution_duration_seconds metric concurrently
			go webhookutils.RegisterPolicyExecutionDurationMetricGenerate(context.TODO(), h.log, h.metrics, string(request.Operation), policy, *engineResponse)
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

	if request.Operation == admissionv1.Update {
		h.handleUpdatesForGenerateRules(ctx, request, policies)
	}
}

// handleUpdatesForGenerateRules handles admission-requests for update
func (h *generationHandler) handleUpdatesForGenerateRules(ctx context.Context, request *admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface) {
	if request.Operation != admissionv1.Update {
		return
	}

	resource, err := kubeutils.BytesToUnstructured(request.OldObject.Raw)
	if err != nil {
		h.log.Error(err, "failed to convert object resource to unstructured format")
	}

	resLabels := resource.GetLabels()
	if resLabels[generate.LabelClonePolicyName] != "" {
		h.handleUpdateGenerateSourceResource(ctx, resLabels)
	}

	if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp && resLabels[generate.LabelSynchronize] == "enable" && request.Operation == admissionv1.Update {
		h.handleUpdateGenerateTargetResource(ctx, resource, policies, resLabels)
	}
}

// handleUpdateGenerateSourceResource - handles update of clone source for generate policy
func (h *generationHandler) handleUpdateGenerateSourceResource(ctx context.Context, resLabels map[string]string) {
	policyNames := strings.Split(resLabels[generate.LabelClonePolicyName], ",")
	for _, policyName := range policyNames {
		// check if the policy exists
		_, err := h.kyvernoClient.KyvernoV1().ClusterPolicies().Get(ctx, policyName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				h.log.V(4).Info("skipping update of update request as policy is deleted")
			} else {
				h.log.Error(err, "failed to get generate policy", "Name", policyName)
			}
		} else {
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				kyvernov1beta1.URGeneratePolicyLabel: policyName,
			}))

			urList, err := h.urLister.List(selector)
			if err != nil {
				h.log.Error(err, "failed to get update request for the resource", "label", kyvernov1beta1.URGeneratePolicyLabel)
				return
			}

			for _, ur := range urList {
				h.urUpdater.UpdateAnnotation(h.log, ur.GetName())
			}
		}
	}
}

// handleUpdateGenerateTargetResource - handles update of target resource for generate policy
func (h *generationHandler) handleUpdateGenerateTargetResource(ctx context.Context, newRes *unstructured.Unstructured, policies []kyvernov1.PolicyInterface, resLabels map[string]string) {
	enqueueBool := false

	policyName := resLabels[generate.LabelDataPolicyName]
	targetSourceName := newRes.GetName()
	targetSourceKind := newRes.GetKind()

	policy, err := h.kyvernoClient.KyvernoV1().ClusterPolicies().Get(ctx, policyName, metav1.GetOptions{})
	if err != nil {
		h.log.Error(err, "failed to get policy from kyverno client.", "policy name", policyName)
		return
	}

	for _, rule := range autogen.ComputeRules(policy) {
		if rule.Generation.Kind == targetSourceKind && rule.Generation.Name == targetSourceName {
			updatedRule, err := getGeneratedByResource(ctx, newRes, resLabels, h.client, rule, h.log)
			if err != nil {
				h.log.V(4).Info("skipping generate policy and resource pattern validation", "error", err)
			} else {
				data := updatedRule.Generation.DeepCopy().GetData()
				if data != nil {
					if _, err := gen.ValidateResourceWithPattern(h.log, newRes.Object, data); err != nil {
						enqueueBool = true
						break
					}
				}

				cloneName := updatedRule.Generation.Clone.Name
				if cloneName != "" {
					obj, err := h.client.GetResource(ctx, "", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
					if err != nil {
						h.log.Error(err, fmt.Sprintf("source resource %s/%s/%s not found.", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name))
						continue
					}

					sourceObj, newResObj := stripNonPolicyFields(obj.Object, newRes.Object, h.log)

					if _, err := gen.ValidateResourceWithPattern(h.log, newResObj, sourceObj); err != nil {
						enqueueBool = true
						break
					}
				}
			}
		}
	}

	if enqueueBool {
		urName := resLabels[generate.LabelURName]
		ur, err := h.urLister.Get(urName)
		if err != nil {
			h.log.Error(err, "failed to get update request", "name", urName)
			return
		}
		h.urUpdater.UpdateAnnotation(h.log, ur.GetName())
	}
}

func (h *generationHandler) deleteGR(ctx context.Context, engineResponse *engineapi.EngineResponse) {
	h.log.V(4).Info("querying all update requests")
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		kyvernov1beta1.URGeneratePolicyLabel:       engineResponse.Policy.GetName(),
		kyvernov1beta1.URGenerateResourceNameLabel: engineResponse.Resource.GetName(),
		kyvernov1beta1.URGenerateResourceKindLabel: engineResponse.Resource.GetKind(),
		kyvernov1beta1.URGenerateResourceNSLabel:   engineResponse.Resource.GetNamespace(),
	}))

	urList, err := h.urLister.List(selector)
	if err != nil {
		h.log.Error(err, "failed to get update request for the resource", "kind", engineResponse.Resource.GetKind(), "name", engineResponse.Resource.GetName(), "namespace", engineResponse.Resource.GetNamespace())
		return
	}

	for _, v := range urList {
		err := h.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(ctx, v.GetName(), metav1.DeleteOptions{})
		if err != nil {
			h.log.Error(err, "failed to update ur")
		}
	}
}
