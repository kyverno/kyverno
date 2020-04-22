package engine

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//Validate applies validation rules from policy on the resource
func Validate(policyContext PolicyContext) response.EngineResponse {
	startTime := time.Now()
	policy := policyContext.Policy
	newR := policyContext.NewResource
	oldR := policyContext.OldResource
	ctx := policyContext.Context
	admissionInfo := policyContext.AdmissionInfo
	logger := log.Log.WithName("Validate").WithValues("policy", policy.Name, "kind", newR.GetKind(), "namespace", newR.GetNamespace(), "name", newR.GetName())

	// policy information
	logger.V(4).Info("start processing", "startTime", startTime)

	var resp *response.EngineResponse
	// handling delete requests
	if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
		resp = validateResource(logger, ctx, policy, oldR, admissionInfo, true)
	} else {
		resp = validateResource(logger, ctx, policy, newR, admissionInfo, false)
	}

	startResultResponse(resp, policy, newR)
	defer endResultResponse(logger, resp, startTime)
	// set PatchedResource with origin resource if empty
	// in order to create policy violation
	if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
		resp.PatchedResource = newR
	}
	return *resp
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

func endResultResponse(log logr.Logger, resp *response.EngineResponse, startTime time.Time) {
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	log.V(4).Info("finshed processing", "processingTime", resp.PolicyResponse.ProcessingTime, "validationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
}

func incrementAppliedCount(resp *response.EngineResponse) {
	// rules applied successfully count
	resp.PolicyResponse.RulesAppliedCount++
}

func validateResource(log logr.Logger, ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, isDelete bool) *response.EngineResponse {
	resp := &response.EngineResponse{}
	for _, rule := range policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}

		// check if the resource satisfies the filter conditions defined in the rule
		// TODO: this needs to be extracted, to filter the resource so that we can avoid passing resources that
		// dont statisfy a policy rule resource description
		if err := MatchesResourceDescription(resource, rule, admissionInfo); err != nil {
			log.V(4).Info("resource fails the match description")
			continue
		}

		// operate on the copy of the conditions, as we perform variable substitution
		preconditionsCopy := copyConditions(rule.Conditions)
		// evaluate pre-conditions
		// - handle variable subsitutions
		if !variables.EvaluateConditions(log, ctx, preconditionsCopy) {
			log.V(4).Info("resource fails the preconditions")
			continue
		}

		if rule.Validation.Deny != nil {
			denyConditionsCopy := copyConditions(rule.Validation.Deny.Conditions)
			if rule.Validation.Deny.AllRequests || !variables.EvaluateConditions(log, ctx, denyConditionsCopy) {
				ruleResp := response.RuleResponse{
					Name:    rule.Name,
					Type:    utils.Validation.String(),
					Message: rule.Validation.Message,
					Success: false,
				}
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResp)
			}
			continue
		}

		if !isDelete {
			if rule.Validation.Pattern != nil || rule.Validation.AnyPattern != nil {
				ruleResponse := validatePatterns(log, ctx, resource, rule)
				incrementAppliedCount(resp)
				resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResponse)
			}
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
func validatePatterns(log logr.Logger, ctx context.EvalInterface, resource unstructured.Unstructured, rule kyverno.Rule) (resp response.RuleResponse) {
	startTime := time.Now()
	logger := log.WithValues("rule", rule.Name)
	logger.V(4).Info("start processing rule", "startTime", startTime)
	resp.Name = rule.Name
	resp.Type = utils.Validation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		logger.V(4).Info("finshed processing", "processingTime", resp.RuleStats.ProcessingTime)
	}()
	// work on a copy of validation rule
	validationRule := rule.Validation.DeepCopy()

	// either pattern or anyPattern can be specified in Validation rule
	if validationRule.Pattern != nil {
		// substitute variables in the pattern
		pattern := validationRule.Pattern
		var err error
		if pattern, err = variables.SubstituteVars(logger, ctx, pattern); err != nil {
			// variable subsitution failed
			resp.Success = false
			resp.Message = fmt.Sprintf("Validation error: %s; Validation rule '%s' failed. '%s'",
				rule.Validation.Message, rule.Name, err)
			return resp
		}

		if path, err := validate.ValidateResourceWithPattern(logger, resource.Object, pattern); err != nil {
			// validation failed
			resp.Success = false
			resp.Message = fmt.Sprintf("Validation error: %s; Validation rule '%s' failed at path '%s'",
				rule.Validation.Message, rule.Name, path)
			return resp
		}
		// rule application successful
		logger.V(4).Info("successfully processed rule")
		resp.Success = true
		resp.Message = fmt.Sprintf("Validation rule '%s' succeeded.", rule.Name)
		return resp
	}

	if validationRule.AnyPattern != nil {
		var failedSubstitutionsErrors []error
		var failedAnyPatternsErrors []error
		var err error
		for idx, pattern := range validationRule.AnyPattern {
			if pattern, err = variables.SubstituteVars(logger, ctx, pattern); err != nil {
				// variable subsitution failed
				failedSubstitutionsErrors = append(failedSubstitutionsErrors, err)
				continue
			}
			_, err := validate.ValidateResourceWithPattern(logger, resource.Object, pattern)
			if err == nil {
				resp.Success = true
				resp.Message = fmt.Sprintf("Validation rule '%s' anyPattern[%d] succeeded.", rule.Name, idx)
				return resp
			}
			logger.V(4).Info(fmt.Sprintf("validation rule failed for anyPattern[%d]", idx), "message", rule.Validation.Message)
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
			log.V(4).Info(fmt.Sprintf("Validation rule '%s' failed. %s", rule.Name, errorStr))
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
