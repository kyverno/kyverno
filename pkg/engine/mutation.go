package engine

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/mutate"
	"github.com/nirmata/kyverno/pkg/engine/rbac"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	PodControllers           = "DaemonSet,Deployment,Job,StatefulSet"
	PodControllersAnnotation = "pod-policies.kyverno.io/autogen-controllers"
	PodTemplateAnnotation    = "pod-policies.kyverno.io/autogen-applied"
)

// Mutate performs mutation. Overlay first and then mutation patches
func Mutate(policyContext PolicyContext) (resp response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy
	resource := policyContext.NewResource
	ctx := policyContext.Context

	startMutateResultResponse(&resp, policy, resource)
	glog.V(4).Infof("started applying mutation rules of policy %q (%v)", policy.Name, startTime)
	defer endMutateResultResponse(&resp, startTime)

	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		resp.PolicyResponse.RulesAppliedCount++
	}

	patchedResource := policyContext.NewResource

	for _, rule := range policy.Spec.Rules {
		//TODO: to be checked before calling the resources as well
		if !rule.HasMutate() && !strings.Contains(PodControllers, resource.GetKind()) {
			continue
		}

		if paths := validateGeneralRuleInfoVariables(ctx, rule); len(paths) != 0 {
			glog.Infof("referenced path not present in rule %s, resource %s/%s/%s, path: %s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), paths)
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules,
				newPathNotPresentRuleResponse(rule.Name, utils.Mutation.String(), fmt.Sprintf("path not present in rule info: %s", paths)))
			continue
		}

		startTime := time.Now()
		if !rbac.MatchAdmissionInfo(rule, policyContext.AdmissionInfo) {
			glog.V(3).Infof("rule '%s' cannot be applied on %s/%s/%s, admission permission: %v",
				rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), policyContext.AdmissionInfo)
			continue
		}
		glog.V(4).Infof("Time: Mutate matchAdmissionInfo %v", time.Since(startTime))

		// check if the resource satisfies the filter conditions defined in the rule
		//TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		ok := MatchesResourceDescription(resource, rule)
		if !ok {
			glog.V(4).Infof("resource %s/%s does not satisfy the resource description for the rule ", resource.GetNamespace(), resource.GetName())
			continue
		}

		// evaluate pre-conditions
		if !variables.EvaluateConditions(ctx, rule.Conditions) {
			glog.V(4).Infof("resource %s/%s does not satisfy the conditions for the rule ", resource.GetNamespace(), resource.GetName())
			continue
		}

		// Process Overlay
		if rule.Mutation.Overlay != nil {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = mutate.ProcessOverlay(ctx, rule, patchedResource)
			if ruleResponse.Success == true {
				// - variable substitution path is not present
				if ruleResponse.PathNotPresent {
					glog.V(4).Infof(ruleResponse.Message)
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
					continue
				}

				// - overlay pattern does not match the resource conditions
				if ruleResponse.Patches == nil {
					glog.V(4).Infof(ruleResponse.Message)
					continue
				}

				glog.V(4).Infof("Mutate overlay in rule '%s' successfully applied on %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
			}

			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount()
		}

		// Process Patches
		if rule.Mutation.Patches != nil {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = mutate.ProcessPatches(rule, patchedResource)
			glog.Infof("Mutate patches in rule '%s' successfully applied on %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			incrementAppliedRuleCount()
		}

		// insert annotation to podtemplate if resource is pod controller
		// skip inserting on existing resource
		if reflect.DeepEqual(policyContext.AdmissionInfo, kyverno.RequestInfo{}) {
			continue
		}

		if strings.Contains(PodControllers, resource.GetKind()) {
			var ruleResponse response.RuleResponse
			ruleResponse, patchedResource = mutate.ProcessOverlay(ctx, podTemplateRule, patchedResource)
			if !ruleResponse.Success {
				glog.Errorf("Failed to insert annotation to podTemplate of %s/%s/%s: %s", resource.GetKind(), resource.GetNamespace(), resource.GetName(), ruleResponse.Message)
				continue
			}

			if ruleResponse.Success && ruleResponse.Patches != nil {
				glog.V(2).Infof("Inserted annotation to podTemplate of %s/%s/%s: %s", resource.GetKind(), resource.GetNamespace(), resource.GetName(), ruleResponse.Message)
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			}
		}
	}
	// send the patched resource
	resp.PatchedResource = patchedResource
	return resp
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

func endMutateResultResponse(resp *response.EngineResponse, startTime time.Time) {
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	glog.V(4).Infof("finished applying mutation rules policy %v (%v)", resp.PolicyResponse.Policy, resp.PolicyResponse.ProcessingTime)
	glog.V(4).Infof("Mutation Rules appplied count %v for policy %q", resp.PolicyResponse.RulesAppliedCount, resp.PolicyResponse.Policy)
}

// podTemplateRule mutate pod template with annotation
// pod-policies.kyverno.io/autogen-applied=true
var podTemplateRule = kyverno.Rule{
	Name: "autogen-annotate-podtemplate",
	Mutation: kyverno.Mutation{
		Overlay: map[string]interface{}{
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"+(pod-policies.kyverno.io/autogen-applied)": "true",
						},
					},
				},
			},
		},
	},
}
