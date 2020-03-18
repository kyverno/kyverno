package engine

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
		// set PatchedResource with origin resource if empty
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
	glog.V(4).Infof("Validation Rules appplied successfully count %v for policy %q", resp.PolicyResponse.RulesAppliedCount, resp.PolicyResponse.Policy)
}

func incrementAppliedCount(resp *response.EngineResponse) {
	// rules applied successfully count
	resp.PolicyResponse.RulesAppliedCount++
}

func validateResource(ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo) *response.EngineResponse {
	resp := &response.EngineResponse{}
	for _, rule := range policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}
		startTime := time.Now()
		glog.V(4).Infof("Time: Validate matchAdmissionInfo %v", time.Since(startTime))

		// check if the resource satisfies the filter conditions defined in the rule
		// TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		if err := MatchesResourceDescription(resource, rule, admissionInfo); err != nil {
			glog.V(4).Infof("resource %s/%s does not satisfy the resource description for the rule:\n%s", resource.GetNamespace(), resource.GetName(), err.Error())
			continue
		}

		// operate on the copy of the conditions, as we perform variable substitution
		copyConditions := copyConditions(rule.Conditions)
		// evaluate pre-conditions
		// - handle variable subsitutions
		if !variables.EvaluateConditions(ctx, copyConditions) {
			glog.V(4).Infof("resource %s/%s does not satisfy the conditions for the rule ", resource.GetNamespace(), resource.GetName())
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
	// if the response are same then return true
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
	resp.Type = utils.Validation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying validation rule %q (%v)", resp.Name, resp.RuleStats.ProcessingTime)
	}()
	// work on a copy of validation rule
	validationRule := rule.Validation.DeepCopy()

	// either pattern or anyPattern can be specified in Validation rule
	if validationRule.Pattern != nil {
		// substitute variables in the pattern
		pattern := validationRule.Pattern
		var err error
		if pattern, err = variables.SubstituteVars(ctx, pattern); err != nil {
			// variable subsitution failed
			resp.Success = false
			resp.Message = fmt.Sprintf("Validation error: %s; Validation rule '%s' failed. '%s'",
				rule.Validation.Message, rule.Name, err)
			return resp
		}

		if path, err := validate.ValidateResourceWithPattern(resource.Object, pattern); err != nil {
			// validation failed
			resp.Success = false
			resp.Message = fmt.Sprintf("Validation error: %s; Validation rule '%s' failed at path '%s'",
				rule.Validation.Message, rule.Name, path)
			return resp
		}
		// rule application successful
		glog.V(4).Infof("rule %s pattern validated successfully on resource %s/%s/%s", rule.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName())
		resp.Success = true
		resp.Message = fmt.Sprintf("Validation rule '%s' succeeded.", rule.Name)
		return resp
	}

	if validationRule.AnyPattern != nil {
		var failedSubstitutionsErrors []error
		var failedAnyPatternsErrors []error
		var err error
		for idx, pattern := range validationRule.AnyPattern {
			if pattern, err = variables.SubstituteVars(ctx, pattern); err != nil {
				// variable subsitution failed
				failedSubstitutionsErrors = append(failedSubstitutionsErrors, err)
				continue
			}
			_, err := validate.ValidateResourceWithPattern(resource.Object, pattern)
			if err == nil {
				resp.Success = true
				resp.Message = fmt.Sprintf("Validation rule '%s' anyPattern[%d] succeeded.", rule.Name, idx)
				return resp
			}
			glog.V(4).Infof("Validation error: %s; Validation rule %s anyPattern[%d] for %s/%s/%s",
				rule.Validation.Message, rule.Name, idx, resource.GetKind(), resource.GetNamespace(), resource.GetName())
			patternErr := fmt.Errorf("anyPattern[%d] failed; %s", idx, err)
			failedAnyPatternsErrors = append(failedAnyPatternsErrors, patternErr)
		}

		// Subsitution falures
		if len(failedSubstitutionsErrors) > 0 {
			resp.Success = false
			resp.Message = fmt.Sprintf("Substitutions failed: %v", failedSubstitutionsErrors)
			return resp
		}

		// Any Pattern validation errors
		if len(failedAnyPatternsErrors) > 0 {
			var errorStr []string
			for _, err := range failedAnyPatternsErrors {
				errorStr = append(errorStr, err.Error())
			}
			resp.Success = false
			glog.V(4).Infof("Validation rule '%s' failed. %s", rule.Name, errorStr)
			if rule.Validation.Message == "" {
				resp.Message = fmt.Sprintf("Validation rule '%s' has failed", rule.Name)
			} else {
				resp.Message = rule.Validation.Message
			}
			return resp
		}
	}
	return response.RuleResponse{}
}
