package resource

import (
	contextdefault "context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	gencommon "github.com/kyverno/kyverno/pkg/background/common"
	gen "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

//handleGenerate handles admission-requests for policies with generate rules
func (h *handlers) handleGenerate(
	logger logr.Logger,
	request *admissionv1.AdmissionRequest,
	policies []kyverno.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp int64,
	latencySender *chan int64,
	generateEngineResponsesSenderForAdmissionReviewDurationMetric *chan []*response.EngineResponse,
	generateEngineResponsesSenderForAdmissionRequestsCountMetric *chan []*response.EngineResponse,
) {
	logger.V(6).Info("update request")

	var engineResponses []*response.EngineResponse
	if (request.Operation == admissionv1.Create || request.Operation == admissionv1.Update) && len(policies) != 0 {
		for _, policy := range policies {
			var rules []response.RuleResponse
			policyContext.Policy = policy
			if request.Kind.Kind != "Namespace" && request.Namespace != "" {
				policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
			}
			engineResponse := engine.ApplyBackgroundChecks(policyContext)
			for _, rule := range engineResponse.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					h.deleteGR(logger, engineResponse)
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
			go h.registerPolicyResultsMetricGeneration(logger, string(request.Operation), policy, *engineResponse)
			// registering the kyverno_policy_execution_duration_seconds metric concurrently
			go h.registerPolicyExecutionDurationMetricGenerate(logger, string(request.Operation), policy, *engineResponse)
		}

		if failedResponse := applyUpdateRequest(request, urkyverno.Generate, h.urGenerator, policyContext.AdmissionInfo, request.Operation, engineResponses...); failedResponse != nil {
			// report failure event
			for _, failedUR := range failedResponse {
				err := fmt.Errorf("failed to create Update Request: %v", failedUR.err)
				e := event.NewBackgroundFailedEvent(err, failedUR.ur.Policy, "", event.GeneratePolicyController, &policyContext.NewResource)
				h.eventGen.Add(e...)
			}
		}
	}

	if request.Operation == admissionv1.Update {
		h.handleUpdatesForGenerateRules(logger, request, policies)
	}

	// sending the admission request latency to other goroutine (reporting the metrics) over the channel
	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	*latencySender <- admissionReviewLatencyDuration
	*generateEngineResponsesSenderForAdmissionReviewDurationMetric <- engineResponses
	*generateEngineResponsesSenderForAdmissionRequestsCountMetric <- engineResponses
}

//handleUpdatesForGenerateRules handles admission-requests for update
func (h *handlers) handleUpdatesForGenerateRules(logger logr.Logger, request *admissionv1.AdmissionRequest, policies []kyverno.PolicyInterface) {
	if request.Operation != admissionv1.Update {
		return
	}

	resource, err := enginutils.ConvertToUnstructured(request.OldObject.Raw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}

	resLabels := resource.GetLabels()
	if resLabels["generate.kyverno.io/clone-policy-name"] != "" {
		h.handleUpdateGenerateSourceResource(resLabels, logger)
	}

	if resLabels["app.kubernetes.io/managed-by"] == "kyverno" && resLabels["policy.kyverno.io/synchronize"] == "enable" && request.Operation == admissionv1.Update {
		h.handleUpdateGenerateTargetResource(request, policies, resLabels, logger)
	}
}

//handleUpdateGenerateSourceResource - handles update of clone source for generate policy
func (h *handlers) handleUpdateGenerateSourceResource(resLabels map[string]string, logger logr.Logger) {
	policyNames := strings.Split(resLabels["generate.kyverno.io/clone-policy-name"], ",")
	for _, policyName := range policyNames {
		// check if the policy exists
		_, err := h.kyvernoClient.KyvernoV1().ClusterPolicies().Get(contextdefault.TODO(), policyName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.V(4).Info("skipping update of update request as policy is deleted")
			} else {
				logger.Error(err, "failed to get generate policy", "Name", policyName)
			}
		} else {
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				urkyverno.URGeneratePolicyLabel: policyName,
			}))

			urList, err := h.urLister.List(selector)
			if err != nil {
				logger.Error(err, "failed to get update request for the resource", "label", urkyverno.URGeneratePolicyLabel)
				return
			}

			for _, ur := range urList {
				h.updateAnnotationInUR(ur, logger)
			}
		}
	}
}

// updateAnnotationInUR - function used to update UR annotation
// updating UR will trigger reprocessing of UR and recreation/updation of generated resource
func (h *handlers) updateAnnotationInUR(ur *urkyverno.UpdateRequest, logger logr.Logger) {
	urAnnotations := ur.Annotations
	if len(urAnnotations) == 0 {
		urAnnotations = make(map[string]string)
	}
	h.mu.Lock()
	urAnnotations["generate.kyverno.io/updation-time"] = time.Now().String()
	ur.SetAnnotations(urAnnotations)
	h.mu.Unlock()

	patch := jsonutils.NewPatch(
		"/metadata/annotations",
		"replace",
		ur.Annotations,
	)

	new, err := gencommon.PatchUpdateRequest(ur, patch, h.kyvernoClient)
	if err != nil {
		logger.Error(err, "failed to update update request update-time annotations for the resource", "update request", ur.Name)
		return
	}
	new.Status.State = urkyverno.Pending
	if _, err := h.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(contextdefault.TODO(), new, metav1.UpdateOptions{}); err != nil {
		logger.Error(err, "failed to set UpdateRequest state to Pending", "update request", ur.Name)
	}
}

//handleUpdateGenerateTargetResource - handles update of target resource for generate policy
func (h *handlers) handleUpdateGenerateTargetResource(request *admissionv1.AdmissionRequest, policies []kyverno.PolicyInterface, resLabels map[string]string, logger logr.Logger) {
	enqueueBool := false
	newRes, err := enginutils.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}

	policyName := resLabels["policy.kyverno.io/policy-name"]
	targetSourceName := newRes.GetName()
	targetSourceKind := newRes.GetKind()

	policy, err := h.kyvernoClient.KyvernoV1().ClusterPolicies().Get(contextdefault.TODO(), policyName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "failed to get policy from kyverno client.", "policy name", policyName)
		return
	}

	for _, rule := range autogen.ComputeRules(policy) {
		if rule.Generation.Kind == targetSourceKind && rule.Generation.Name == targetSourceName {
			updatedRule, err := getGeneratedByResource(newRes, resLabels, h.client, rule, logger)
			if err != nil {
				logger.V(4).Info("skipping generate policy and resource pattern validaton", "error", err)
			} else {
				data := updatedRule.Generation.DeepCopy().GetData()
				if data != nil {
					if _, err := gen.ValidateResourceWithPattern(logger, newRes.Object, data); err != nil {
						enqueueBool = true
						break
					}
				}

				cloneName := updatedRule.Generation.Clone.Name
				if cloneName != "" {
					obj, err := h.client.GetResource("", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
					if err != nil {
						logger.Error(err, fmt.Sprintf("source resource %s/%s/%s not found.", rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name))
						continue
					}

					sourceObj, newResObj := stripNonPolicyFields(obj.Object, newRes.Object, logger)

					if _, err := gen.ValidateResourceWithPattern(logger, newResObj, sourceObj); err != nil {
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
			logger.Error(err, "failed to get update request", "name", urName)
			return
		}
		h.updateAnnotationInUR(ur, logger)
	}
}

func (h *handlers) deleteGR(logger logr.Logger, engineResponse *response.EngineResponse) {
	logger.V(4).Info("querying all update requests")
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		urkyverno.URGeneratePolicyLabel:          engineResponse.PolicyResponse.Policy.Name,
		"generate.kyverno.io/resource-name":      engineResponse.PolicyResponse.Resource.Name,
		"generate.kyverno.io/resource-kind":      engineResponse.PolicyResponse.Resource.Kind,
		"generate.kyverno.io/resource-namespace": engineResponse.PolicyResponse.Resource.Namespace,
	}))

	urList, err := h.urLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to get update request for the resource", "kind", engineResponse.PolicyResponse.Resource.Kind, "name", engineResponse.PolicyResponse.Resource.Name, "namespace", engineResponse.PolicyResponse.Resource.Namespace)
		return
	}

	for _, v := range urList {
		err := h.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(contextdefault.TODO(), v.GetName(), metav1.DeleteOptions{})
		if err != nil {
			logger.Error(err, "failed to update ur")
		}
	}
}

// type updateRequestResponse struct {
// 	ur  urkyverno.UpdateRequestSpec
// 	err error
// }
