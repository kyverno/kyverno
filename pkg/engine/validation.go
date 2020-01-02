package engine

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func startResultResponse(resp *response.EngineResponse, policy kyverno.ClusterPolicy, newR unstructured.Unstructured) {
	// set policy information
	resp.PolicyResponse.Policy = policy.Name
	// resource details
	resp.PolicyResponse.Resource.Name = newR.GetName()
	resp.PolicyResponse.Resource.Namespace = newR.GetNamespace()
	resp.PolicyResponse.Resource.Kind = newR.GetKind()
	resp.PolicyResponse.Resource.APIVersion = newR.GetAPIVersion()
	resp.PolicyResponse.ValidationFailureAction = policy.Spec.ValidationFailureAction

}

func endResultResponse(resp *response.EngineResponse, startTime time.Time) {
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	glog.V(4).Infof("Finished applying validation rules policy %v (%v)", resp.PolicyResponse.Policy, resp.PolicyResponse.ProcessingTime)
	glog.V(4).Infof("Validation Rules appplied succesfully count %v for policy %q", resp.PolicyResponse.RulesAppliedCount, resp.PolicyResponse.Policy)
}

func incrementAppliedCount(resp *response.EngineResponse) {
	// rules applied succesfully count
	resp.PolicyResponse.RulesAppliedCount++
}

//Validate applies validation rules from policy on the resource
func Validate(policyContext PolicyContext) (resp response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy
	newR := policyContext.NewResource
	oldR := policyContext.OldResource
	ctx := policyContext.Context
	admissionInfo := policyContext.AdmissionInfo

	// policy information
	glog.V(4).Infof("started applying validation rules of policy %q (%v)", policy.Name, startTime)

	// Process new & old resource
	if reflect.DeepEqual(oldR, unstructured.Unstructured{}) {
		// Create Mode
		// Operate on New Resource only
		resp := validateResource(ctx, policy, newR, admissionInfo)
		startResultResponse(resp, policy, newR)
		defer endResultResponse(resp, startTime)
		// set PatchedResource with orgin resource if empty
		// in order to create policy violation
		if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
			resp.PatchedResource = newR
		}
		return *resp
	}
	// Update Mode
	// Operate on New and Old Resource only
	// New resource
	oldResponse := validateResource(ctx, policy, oldR, admissionInfo)
	newResponse := validateResource(ctx, policy, newR, admissionInfo)

	// if the old and new response is same then return empty response
	if !isSameResponse(oldResponse, newResponse) {
		// there are changes send response
		startResultResponse(newResponse, policy, newR)
		defer endResultResponse(newResponse, startTime)
		if reflect.DeepEqual(newResponse.PatchedResource, unstructured.Unstructured{}) {
			newResponse.PatchedResource = newR
		}
		return *newResponse
	}
	// if there are no changes with old and new response then sent empty response
	// skip processing
	return response.EngineResponse{}
}

func validateResource(ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo RequestInfo) *response.EngineResponse {
	resp := &response.EngineResponse{}
	for _, rule := range policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}
		startTime := time.Now()
		if !matchAdmissionInfo(rule, admissionInfo) {
			glog.V(3).Infof("rule '%s' cannot be applied on %s/%s/%s, admission permission: %v",
				rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), admissionInfo)
			continue
		}
		glog.V(4).Infof("Time: Validate matchAdmissionInfo %v", time.Since(startTime))

		// check if the resource satisfies the filter conditions defined in the rule
		// TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		ok := MatchesResourceDescription(resource, rule)
		if !ok {
			glog.V(4).Infof("resource %s/%s does not satisfy the resource description for the rule ", resource.GetNamespace(), resource.GetName())
			continue
		}
		if rule.Validation.Pattern != nil || rule.Validation.AnyPattern != nil {
			ruleResponse := validatePatterns(ctx, resource, rule)
			incrementAppliedCount(resp)
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
		}
	}
	return resp
}

func isSameResponse(oldResponse, newResponse *response.EngineResponse) bool {
	// if the respones are same then return true
	return isSamePolicyResponse(oldResponse.PolicyResponse, newResponse.PolicyResponse)

}

func isSamePolicyResponse(oldPolicyRespone, newPolicyResponse response.PolicyResponse) bool {
	// can skip policy and resource checks as they will be same
	// compare rules
	return isSameRules(oldPolicyRespone.Rules, newPolicyResponse.Rules)
}

func isSameRules(oldRules []response.RuleResponse, newRules []response.RuleResponse) bool {
	if len(oldRules) != len(newRules) {
		return false
	}
	// as the rules are always processed in order the indices wil be same
	for idx, oldrule := range oldRules {
		newrule := newRules[idx]
		// Name
		if oldrule.Name != newrule.Name {
			return false
		}
		// Type
		if oldrule.Type != newrule.Type {
			return false
		}
		// Message
		if oldrule.Message != newrule.Message {
			return false
		}
		// skip patches
		if oldrule.Success != newrule.Success {
			return false
		}
	}
	return true
}

// validatePatterns validate pattern and anyPattern
func validatePatterns(ctx context.EvalInterface, resource unstructured.Unstructured, rule kyverno.Rule) (resp response.RuleResponse) {
	startTime := time.Now()
	glog.V(4).Infof("started applying validation rule %q (%v)", rule.Name, startTime)
	resp.Name = rule.Name
	resp.Type = Validation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying validation rule %q (%v)", resp.Name, resp.RuleStats.ProcessingTime)
	}()

	// either pattern or anyPattern can be specified in Validation rule
	if rule.Validation.Pattern != nil {
		path, err := validate.ValidateResourceWithPattern(ctx, resource.Object, rule.Validation.Pattern)
		if err != nil {
			// rule application failed
			glog.V(4).Infof("Validation rule '%s' failed at '%s' for resource %s/%s/%s. %s: %v", rule.Name, path, resource.GetKind(), resource.GetNamespace(), resource.GetName(), rule.Validation.Message, err)
			resp.Success = false
			resp.Message = fmt.Sprintf("Validation error: %s; Validation rule '%s' failed at path '%s'",
				rule.Validation.Message, rule.Name, path)
			return resp
		}
		// rule application succesful
		glog.V(4).Infof("rule %s pattern validated succesfully on resource %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
		resp.Success = true
		resp.Message = fmt.Sprintf("Validation rule '%s' succeeded.", rule.Name)
		return resp
	}

	// using anyPattern we can define multiple patterns and only one of them has to be succesfully validated
	if rule.Validation.AnyPattern != nil {
		var errs []error
		var failedPaths []string
		for index, pattern := range rule.Validation.AnyPattern {
			path, err := validate.ValidateResourceWithPattern(ctx, resource.Object, pattern)
			if err == nil {
				// this pattern was succesfully validated
				glog.V(4).Infof("anyPattern %v succesfully validated on resource %s/%s/%s", pattern, resource.GetKind(), resource.GetNamespace(), resource.GetName())
				resp.Success = true
				resp.Message = fmt.Sprintf("Validation rule '%s' anyPattern[%d] succeeded.", rule.Name, index)
				return resp
			}
			if err != nil {
				glog.V(4).Infof("Validation error: %s; Validation rule %s anyPattern[%d] failed at path %s for %s/%s/%s",
					rule.Validation.Message, rule.Name, index, path, resource.GetKind(), resource.GetNamespace(), resource.GetName())
				errs = append(errs, err)
				failedPaths = append(failedPaths, path)
			}
		}
		// If none of the anyPatterns are validated
		if len(errs) > 0 {
			glog.V(4).Infof("none of anyPattern were processed: %v", errs)
			resp.Success = false
			var errorStr []string
			for index, err := range errs {
				glog.V(4).Infof("anyPattern[%d] failed at path %s: %v", index, failedPaths[index], err)
				str := fmt.Sprintf("Validation rule %s anyPattern[%d] failed at path %s.", rule.Name, index, failedPaths[index])
				errorStr = append(errorStr, str)
			}
			resp.Message = fmt.Sprintf("Validation error: %s; %s", rule.Validation.Message, strings.Join(errorStr, ";"))

			return resp
		}
	}
	return response.RuleResponse{}
}
