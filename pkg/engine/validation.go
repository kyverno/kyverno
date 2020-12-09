package engine

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//Validate applies validation rules from policy on the resource
func Validate(policyContext PolicyContext) (resp response.EngineResponse) {
	startTime := time.Now()
	policy := policyContext.Policy
	newR := policyContext.NewResource
	oldR := policyContext.OldResource
	ctx := policyContext.Context
	admissionInfo := policyContext.AdmissionInfo

	resCache := policyContext.ResourceCache
	jsonContext := policyContext.JSONContext
	logger := log.Log.WithName("EngineValidate").WithValues("policy", policy.Name)

	if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
		logger = logger.WithValues("kind", oldR.GetKind(), "namespace", oldR.GetNamespace(), "name", oldR.GetName())
	} else {
		logger = logger.WithValues("kind", newR.GetKind(), "namespace", newR.GetNamespace(), "name", newR.GetName())
	}

	logger.V(4).Info("start processing", "startTime", startTime)
	defer func() {
		if reflect.DeepEqual(resp, response.EngineResponse{}) {
			return
		}
		var resource unstructured.Unstructured
		if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
			// for delete requests patched resource will be oldR since newR is empty
			if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
				resource = oldR
			} else {
				resource = newR
			}
		}
		for i := range resp.PolicyResponse.Rules {
			messageInterface, err := variables.SubstituteVars(logger, ctx, resp.PolicyResponse.Rules[i].Message)
			if err != nil {
				logger.V(4).Info("failed to substitute variables", "error", err.Error())
				continue
			}
			resp.PolicyResponse.Rules[i].Message, _ = messageInterface.(string)
		}
		resp.PatchedResource = resource
		startResultResponse(&resp, policy, resource)
		endResultResponse(logger, &resp, startTime)
	}()

	// If request is delete, newR will be empty
	if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
		return *isRequestDenied(logger, ctx, policy, oldR, admissionInfo, policyContext.ExcludeGroupRole, resCache, jsonContext)
	}

	if denyResp := isRequestDenied(logger, ctx, policy, newR, admissionInfo, policyContext.ExcludeGroupRole, resCache, jsonContext); !denyResp.IsSuccessful() {
		return *denyResp
	}
	if reflect.DeepEqual(oldR, unstructured.Unstructured{}) {
		return *validateResource(logger, ctx, policy, newR, admissionInfo, policyContext.ExcludeGroupRole, resCache, jsonContext)
	}

	oldResponse := validateResource(logger, ctx, policy, oldR, admissionInfo, policyContext.ExcludeGroupRole, resCache, jsonContext)
	newResponse := validateResource(logger, ctx, policy, newR, admissionInfo, policyContext.ExcludeGroupRole, resCache, jsonContext)
	if !isSameResponse(oldResponse, newResponse) {
		return *newResponse
	}
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

func endResultResponse(log logr.Logger, resp *response.EngineResponse, startTime time.Time) {
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	log.V(4).Info("finished processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "validationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
}

func incrementAppliedCount(resp *response.EngineResponse) {
	// rules applied successfully count
	resp.PolicyResponse.RulesAppliedCount++
}

func isRequestDenied(log logr.Logger, ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, excludeGroupRole []string, resCache resourcecache.ResourceCacheIface, jsonContext *context.Context) *response.EngineResponse {
	resp := &response.EngineResponse{}
	if SkipPolicyApplication(policy, resource) {
		log.V(5).Info("Skip applying policy, Pod has ownerRef set", "policy", policy.GetName())
		return resp
	}
	excludeResource := []string{}
	if len(excludeGroupRole) > 0 {
		excludeResource = excludeGroupRole
	}
	for _, rule := range policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}

		// add configmap json data to context
		if err := AddResourceToContext(log, rule.Context, resCache, jsonContext); err != nil {
			log.V(4).Info("cannot add configmaps to context", "reason", err.Error())
			continue
		}

		if err := MatchesResourceDescription(resource, rule, admissionInfo, excludeResource); err != nil {
			log.V(4).Info("resource fails the match description", "reason", err.Error())
			continue
		}

		preconditionsCopy := copyConditions(rule.Conditions)

		if !variables.EvaluateConditions(log, ctx, preconditionsCopy) {
			log.V(4).Info("resource fails the preconditions")
			continue
		}

		if rule.Validation.Deny != nil {
			denyConditionsCopy := copyConditions(rule.Validation.Deny.Conditions)
			if len(rule.Validation.Deny.Conditions) == 0 || variables.EvaluateConditions(log, ctx, denyConditionsCopy) {
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

	}
	return resp
}

func validateResource(log logr.Logger, ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured, admissionInfo kyverno.RequestInfo, excludeGroupRole []string, resCache resourcecache.ResourceCacheIface, jsonContext *context.Context) *response.EngineResponse {
	resp := &response.EngineResponse{}

	if SkipPolicyApplication(policy, resource) {
		log.V(5).Info("Skip applying policy, Pod has ownerRef set", "policy", policy.GetName())
		return resp
	}

	excludeResource := []string{}
	if len(excludeGroupRole) > 0 {
		excludeResource = excludeGroupRole
	}

	for _, rule := range policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}

		// check if the resource satisfies the filter conditions defined in the rule
		if err := MatchesResourceDescription(resource, rule, admissionInfo, excludeResource); err != nil {
			log.V(4).Info("resource fails the match description", "reason", err.Error())
			continue
		}

		// add configmap json data to context
		if err := AddResourceToContext(log, rule.Context, resCache, jsonContext); err != nil {
			log.V(4).Info("cannot add configmaps to context", "reason", err.Error())
			continue
		}

		// operate on the copy of the conditions, as we perform variable substitution
		preconditionsCopy := copyConditions(rule.Conditions)
		// evaluate pre-conditions
		// - handle variable substitutions
		if !variables.EvaluateConditions(log, ctx, preconditionsCopy) {
			log.V(4).Info("resource fails the preconditions")
			continue
		}

		if rule.Validation.Pattern != nil || rule.Validation.AnyPattern != nil {
			ruleResponse := validatePatterns(log, ctx, resource, rule)
			if common.IsConditionalAnchorError(ruleResponse.Message) {
				continue
			}

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
func validatePatterns(log logr.Logger, ctx context.EvalInterface, resource unstructured.Unstructured, rule kyverno.Rule) (resp response.RuleResponse) {
	startTime := time.Now()
	logger := log.WithValues("rule", rule.Name, "name", resource.GetName(), "kind", resource.GetKind())
	logger.V(5).Info("start processing rule", "startTime", startTime)
	resp.Name = rule.Name
	resp.Type = utils.Validation.String()
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		logger.V(4).Info("finished processing rule", "processingTime", resp.RuleStats.ProcessingTime.String())
	}()

	validationRule := rule.Validation.DeepCopy()
	if validationRule.Pattern != nil {
		pattern := validationRule.Pattern
		var err error
		if pattern, err = variables.SubstituteVars(logger, ctx, pattern); err != nil {
			resp.Success = false
			resp.Message = fmt.Sprintf("variable substitution failed for rule %s: %s", rule.Name, err.Error())
			return resp
		}

		if path, err := validate.ValidateResourceWithPattern(logger, resource.Object, pattern); err != nil {
			logger.V(3).Info("validation failed", "path", path, "error", err.Error())
			resp.Success = false
			resp.Message = buildErrorMessage(rule, path)
			return resp
		}

		logger.V(4).Info("successfully processed rule")
		resp.Success = true
		resp.Message = fmt.Sprintf("validation rule '%s' passed.", rule.Name)
		return resp
	}

	if validationRule.AnyPattern != nil {
		var failedSubstitutionsErrors []error
		var failedAnyPatternsErrors []error
		var err error

		anyPatterns, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			resp.Success = false
			resp.Message = fmt.Sprintf("failed to deserialize anyPattern, expected type array: %v", err)
			return resp
		}

		for idx, pattern := range anyPatterns {
			if pattern, err = variables.SubstituteVars(logger, ctx, pattern); err != nil {
				failedSubstitutionsErrors = append(failedSubstitutionsErrors, err)
				continue
			}

			path, err := validate.ValidateResourceWithPattern(logger, resource.Object, pattern)
			if err == nil {
				resp.Success = true
				resp.Message = fmt.Sprintf("validation rule '%s' anyPattern[%d] passed.", rule.Name, idx)
				return resp
			}

			logger.V(4).Info("validation rule failed", "anyPattern[%d]", idx, "path", path)
			patternErr := fmt.Errorf("Rule %s[%d] failed at path %s.", rule.Name, idx, path)
			failedAnyPatternsErrors = append(failedAnyPatternsErrors, patternErr)
		}

		// Substitution failures
		if len(failedSubstitutionsErrors) > 0 {
			resp.Success = false
			resp.Message = fmt.Sprintf("failed to substitute variables: %v", failedSubstitutionsErrors)
			return resp
		}

		// Any Pattern validation errors
		if len(failedAnyPatternsErrors) > 0 {
			var errorStr []string
			for _, err := range failedAnyPatternsErrors {
				errorStr = append(errorStr, err.Error())
			}

			log.V(4).Info(fmt.Sprintf("Validation rule '%s' failed. %s", rule.Name, errorStr))

			resp.Success = false
			resp.Message = buildAnyPatternErrorMessage(rule, errorStr)
			return resp
		}
	}
	return response.RuleResponse{}
}

func buildErrorMessage(rule kyverno.Rule, path string) string {
	if rule.Validation.Message == "" {
		return fmt.Sprintf("validation error: rule %s failed at path %s", rule.Name, path)
	}

	if strings.HasSuffix(rule.Validation.Message, ".") {
		return fmt.Sprintf("validation error: %s Rule %s failed at path %s", rule.Validation.Message, rule.Name, path)
	}

	return fmt.Sprintf("validation error: %s. Rule %s failed at path %s", rule.Validation.Message, rule.Name, path)
}

func buildAnyPatternErrorMessage(rule kyverno.Rule, errors []string) string {
	errStr := strings.Join(errors, " ")
	if rule.Validation.Message == "" {
		return fmt.Sprintf("validation error: %s", errStr)
	}

	if strings.HasSuffix(rule.Validation.Message, ".") {
		return fmt.Sprintf("validation error: %s %s", rule.Validation.Message, errStr)
	}

	return fmt.Sprintf("validation error: %s. %s", rule.Validation.Message, errStr)
}
