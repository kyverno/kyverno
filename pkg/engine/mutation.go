package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/mutate"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	//PodControllers stores the list of Pod-controllers in csv string
	PodControllers = "DaemonSet,Deployment,Job,StatefulSet"
	//PodControllersAnnotation defines the annotation key for Pod-Controllers
	PodControllersAnnotation = "pod-policies.kyverno.io/autogen-controllers"
	//PodTemplateAnnotation defines the annotation key for Pod-Template
	PodTemplateAnnotation = "pod-policies.kyverno.io/autogen-applied"
	PodControllerRuleName = "podControllerAnnotation"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policyContext PolicyContext) (resp response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy
	resource := policyContext.NewResource
	ctx := policyContext.Context
	logger := log.Log.WithName("Mutate").WithValues("policy", policy.Name, "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	logger.V(4).Info("start policy processing", "startTime", startTime)
	startMutateResultResponse(&resp, policy, resource)
	defer endMutateResultResponse(logger, &resp, startTime)

	patchedResource := policyContext.NewResource

	if autoGenAnnotationApplied(patchedResource) && autoGenPolicy(&policy) {
		resp.PatchedResource = patchedResource
		return
	}

	for _, rule := range policy.Spec.Rules {
		var ruleResponse response.RuleResponse
		logger := logger.WithValues("rule", rule.Name)
		//TODO: to be checked before calling the resources as well
		if !rule.HasMutate() && !strings.Contains(PodControllers, resource.GetKind()) {
			continue
		}

		// check if the resource satisfies the filter conditions defined in the rule
		//TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont satisfy a policy rule resource description
		if err := MatchesResourceDescription(resource, rule, policyContext.AdmissionInfo); err != nil {
			logger.V(3).Info("resource not matched", "reason", err.Error())
			continue
		}

		// operate on the copy of the conditions, as we perform variable substitution
		copyConditions := copyConditions(rule.Conditions)
		// evaluate pre-conditions
		// - handle variable subsitutions
		if !variables.EvaluateConditions(logger, ctx, copyConditions) {
			logger.V(3).Info("resource fails the preconditions")
			continue
		}

		mutation := rule.Mutation.DeepCopy()
		// Process Overlay
		if mutation.Overlay != nil {
			overlay := mutation.Overlay
			// subsiitue the variables
			var err error
			if overlay, err = variables.SubstituteVars(logger, ctx, overlay); err != nil {
				// variable subsitution failed
				ruleResponse.Success = false
				ruleResponse.Message = err.Error()
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
				continue
			}

			ruleResponse, patchedResource = mutate.ProcessOverlay(logger, rule.Name, overlay, patchedResource)
			if ruleResponse.Success {
				// - overlay pattern does not match the resource conditions
				if ruleResponse.Patches == nil {
					continue
				}
				logger.V(4).Info("overlay applied succesfully")
			}

			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount(&resp)
		}

		// Process Patches
		if rule.Mutation.Patches != nil {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = mutate.ProcessPatches(logger, rule, patchedResource)
			logger.V(4).Info("patches applied successfully")
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount(&resp)
		}
	}

	// insert annotation to podtemplate if resource is pod controller
	// skip inserting on UPDATE request
	if !reflect.DeepEqual(policyContext.OldResource, unstructured.Unstructured{}) {
		resp.PatchedResource = patchedResource
		return resp
	}

	if autoGenPolicy(&policy) && strings.Contains(PodControllers, resource.GetKind()) {
		if !patchedResourceHasPodControllerAnnotation(patchedResource) {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = mutate.ProcessOverlay(logger, PodControllerRuleName, podTemplateRule.Mutation.Overlay, patchedResource)
			if !ruleResponse.Success {
				logger.Info("failed to insert annotation for podTemplate", "error", ruleResponse.Message)
			} else {
				if ruleResponse.Success && ruleResponse.Patches != nil {
					logger.V(3).Info("inserted annotation for podTemplate")
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
				}
			}
		}
	}

	// send the patched resource
	resp.PatchedResource = patchedResource
	return resp
}

func autoGenPolicy(policy *kyverno.ClusterPolicy) bool {
	annotations := policy.GetObjectMeta().GetAnnotations()
	_, ok := annotations[PodControllersAnnotation]
	return ok
}

func patchedResourceHasPodControllerAnnotation(resource unstructured.Unstructured) bool {
	var podController struct {
		Spec struct {
			Template struct {
				Metadata struct {
					Annotations map[string]interface{} `json:"annotations"`
				} `json:"metadata"`
			} `json:"template"`
		} `json:"spec"`
	}

	resourceRaw, _ := json.Marshal(resource.Object)
	_ = json.Unmarshal(resourceRaw, &podController)

	val, ok := podController.Spec.Template.Metadata.Annotations[PodTemplateAnnotation]

	log.Log.V(4).Info("patchedResourceHasPodControllerAnnotation", "resourceRaw", string(resourceRaw), "val", val, "ok", ok)

	return ok
}
func incrementAppliedRuleCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesAppliedCount++
}

func startMutateResultResponse(resp *response.EngineResponse, policy kyverno.ClusterPolicy, resource unstructured.Unstructured) {
	// set policy information
	resp.PolicyResponse.Policy = policy.Name
	// resource details
	resp.PolicyResponse.Resource.Name = resource.GetName()
	resp.PolicyResponse.Resource.Namespace = resource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resource.GetAPIVersion()
	// TODO(shuting): set response with mutationFailureAction
}

func endMutateResultResponse(logger logr.Logger, resp *response.EngineResponse, startTime time.Time) {
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	logger.V(4).Info("finished processing policy", "processingTime", resp.PolicyResponse.ProcessingTime, "mutationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
}

// podTemplateRule mutate pod template with annotation
// pod-policies.kyverno.io/autogen-applied=true
var podTemplateRule = kyverno.Rule{
	Mutation: kyverno.Mutation{
		Overlay: map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"+(" + PodTemplateAnnotation + ")": "true",
						},
					},
				},
			},
		},
	},
}
