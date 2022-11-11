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
	gen "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type GenerationHandler interface {
	// TODO: why do we need to expose that ?
	HandleUpdatesForGenerateRules(*admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface)
	Handle(
		*metrics.MetricsConfig,
		*admissionv1.AdmissionRequest,
		[]kyvernov1.PolicyInterface,
		*engine.PolicyContext,
		time.Time,
	)
}

func NewGenerationHandler(
	log logr.Logger,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	nsLister corev1listers.NamespaceLister,
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister,
	urGenerator webhookgenerate.Generator,
	urUpdater webhookutils.UpdateRequestUpdater,
	eventGen event.Interface,
) GenerationHandler {
	return &generationHandler{
		log:           log,
		client:        client,
		kyvernoClient: kyvernoClient,
		nsLister:      nsLister,
		urLister:      urLister,
		urGenerator:   urGenerator,
		urUpdater:     urUpdater,
		eventGen:      eventGen,
	}
}

type generationHandler struct {
	log           logr.Logger
	client        dclient.Interface
	kyvernoClient versioned.Interface
	nsLister      corev1listers.NamespaceLister
	urLister      kyvernov1beta1listers.UpdateRequestNamespaceLister
	urGenerator   webhookgenerate.Generator
	urUpdater     webhookutils.UpdateRequestUpdater
	eventGen      event.Interface
}

// Handle handles admission-requests for policies with generate rules
func (h *generationHandler) Handle(
	metricsConfig *metrics.MetricsConfig,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
) {
	h.log.V(6).Info("update request for generate policy")

	var engineResponses []*response.EngineResponse
	if (request.Operation == admissionv1.Create || request.Operation == admissionv1.Update) && len(policies) != 0 {
		for _, policy := range policies {
			var rules []response.RuleResponse
			policyContext.Policy = policy
			if request.Kind.Kind != "Namespace" && request.Namespace != "" {
				policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, h.log)
			}
			engineResponse := engine.ApplyBackgroundChecks(policyContext)
			for _, rule := range engineResponse.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					h.deleteGR(engineResponse)
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
			go webhookutils.RegisterPolicyResultsMetricGeneration(h.log, metricsConfig, string(request.Operation), policy, *engineResponse)
			// registering the kyverno_policy_execution_duration_seconds metric concurrently
			go webhookutils.RegisterPolicyExecutionDurationMetricGenerate(h.log, metricsConfig, string(request.Operation), policy, *engineResponse)
		}

		if failedResponse := applyUpdateRequest(request, kyvernov1beta1.Generate, h.urGenerator, policyContext.AdmissionInfo, request.Operation, engineResponses...); failedResponse != nil {
			// report failure event
			for _, failedUR := range failedResponse {
				err := fmt.Errorf("failed to create Update Request: %v", failedUR.err)
				e := event.NewBackgroundFailedEvent(err, failedUR.ur.Policy, "", event.GeneratePolicyController, &policyContext.NewResource)
				h.eventGen.Add(e...)
			}
		}
	}

	if request.Operation == admissionv1.Update {
		h.HandleUpdatesForGenerateRules(request, policies)
	}
}

// HandleUpdatesForGenerateRules handles admission-requests for update
func (h *generationHandler) HandleUpdatesForGenerateRules(request *admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface) {
	if request.Operation != admissionv1.Update {
		return
	}

	resource, err := enginutils.ConvertToUnstructured(request.OldObject.Raw)
	if err != nil {
		h.log.Error(err, "failed to convert object resource to unstructured format")
	}

	resLabels := resource.GetLabels()
	if resLabels["generate.kyverno.io/clone-policy-name"] != "" {
		h.handleUpdateGenerateSourceResource(resLabels)
	}

	if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp && resLabels["policy.kyverno.io/synchronize"] == "enable" && request.Operation == admissionv1.Update {
		h.handleUpdateGenerateTargetResource(request, policies, resLabels)
	}
}

// handleUpdateGenerateSourceResource - handles update of clone source for generate policy
func (h *generationHandler) handleUpdateGenerateSourceResource(resLabels map[string]string) {
	policyNames := strings.Split(resLabels["generate.kyverno.io/clone-policy-name"], ",")
	for _, policyName := range policyNames {
		// check if the policy exists
		_, err := h.kyvernoClient.KyvernoV1().ClusterPolicies().Get(context.TODO(), policyName, metav1.GetOptions{})
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
func (h *generationHandler) handleUpdateGenerateTargetResource(request *admissionv1.AdmissionRequest, policies []kyvernov1.PolicyInterface, resLabels map[string]string) {
	enqueueBool := false
	newRes, err := enginutils.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		h.log.Error(err, "failed to convert object resource to unstructured format")
	}

	policyName := resLabels["policy.kyverno.io/policy-name"]
	targetSourceName := newRes.GetName()
	targetSourceKind := newRes.GetKind()

	policy, err := h.kyvernoClient.KyvernoV1().ClusterPolicies().Get(context.TODO(), policyName, metav1.GetOptions{})
	if err != nil {
		h.log.Error(err, "failed to get policy from kyverno client.", "policy name", policyName)
		return
	}

	for _, rule := range autogen.ComputeRules(policy) {
		if rule.Generation.Kind == targetSourceKind && rule.Generation.Name == targetSourceName {
			updatedRule, err := getGeneratedByResource(newRes, resLabels, h.client, rule, h.log)
			if err != nil {
				h.log.V(4).Info("skipping generate policy and resource pattern validaton", "error", err)
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
					obj, err := h.client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
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
		urName := resLabels["policy.kyverno.io/gr-name"]
		ur, err := h.urLister.Get(urName)
		if err != nil {
			h.log.Error(err, "failed to get update request", "name", urName)
			return
		}
		h.urUpdater.UpdateAnnotation(h.log, ur.GetName())
	}
}

func (h *generationHandler) deleteGR(engineResponse *response.EngineResponse) {
	h.log.V(4).Info("querying all update requests")
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		kyvernov1beta1.URGeneratePolicyLabel:       engineResponse.PolicyResponse.Policy.Name,
		kyvernov1beta1.URGenerateResourceNameLabel: engineResponse.PolicyResponse.Resource.Name,
		kyvernov1beta1.URGenerateResourceKindLabel: engineResponse.PolicyResponse.Resource.Kind,
		kyvernov1beta1.URGenerateResourceNSLabel:   engineResponse.PolicyResponse.Resource.Namespace,
	}))

	urList, err := h.urLister.List(selector)
	if err != nil {
		h.log.Error(err, "failed to get update request for the resource", "kind", engineResponse.PolicyResponse.Resource.Kind, "name", engineResponse.PolicyResponse.Resource.Name, "namespace", engineResponse.PolicyResponse.Resource.Namespace)
		return
	}

	for _, v := range urList {
		err := h.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), v.GetName(), metav1.DeleteOptions{})
		if err != nil {
			h.log.Error(err, "failed to update ur")
		}
	}
}
