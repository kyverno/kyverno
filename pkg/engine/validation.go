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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//Validate applies validation rules from policy on the resource
func Validate(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	startTime := time.Now()

	logger := buildLogger(policyContext)
	logger.V(4).Info("start policy processing", "startTime", startTime)
	defer func() {
		buildResponse(logger, policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "validationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	resp = validateResource(logger, policyContext)
	return
}

func buildLogger(ctx *PolicyContext) logr.Logger {
	logger := log.Log.WithName("EngineValidate").WithValues("policy", ctx.Policy.Name)
	if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
		logger = logger.WithValues("kind", ctx.OldResource.GetKind(), "namespace", ctx.OldResource.GetNamespace(), "name", ctx.OldResource.GetName())
	} else {
		logger = logger.WithValues("kind", ctx.NewResource.GetKind(), "namespace", ctx.NewResource.GetNamespace(), "name", ctx.NewResource.GetName())
	}

	return logger
}

func buildResponse(logger logr.Logger, ctx *PolicyContext, resp *response.EngineResponse, startTime time.Time) {
	if reflect.DeepEqual(resp, response.EngineResponse{}) {
		return
	}

	if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
		// for delete requests patched resource will be oldResource since newResource is empty
		var resource unstructured.Unstructured = ctx.NewResource
		if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
			resource = ctx.OldResource
		}

		resp.PatchedResource = resource
	}

	for i := range resp.PolicyResponse.Rules {
		messageInterface, err := variables.SubstituteVars(logger, ctx.JSONContext, resp.PolicyResponse.Rules[i].Message)
		if err != nil {
			logger.V(4).Info("failed to substitute variables", "error", err.Error())
			continue
		}

		resp.PolicyResponse.Rules[i].Message, _ = messageInterface.(string)
	}

	resp.PolicyResponse.Policy = ctx.Policy.Name
	resp.PolicyResponse.Resource.Name = resp.PatchedResource.GetName()
	resp.PolicyResponse.Resource.Namespace = resp.PatchedResource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resp.PatchedResource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resp.PatchedResource.GetAPIVersion()
	resp.PolicyResponse.ValidationFailureAction = ctx.Policy.Spec.ValidationFailureAction
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
}

func incrementAppliedCount(resp *response.EngineResponse) {
	resp.PolicyResponse.RulesAppliedCount++
}

func validateResource(log logr.Logger, ctx *PolicyContext) *response.EngineResponse {
	resp := &response.EngineResponse{}
	if ManagedPodResource(ctx.Policy, ctx.NewResource) {
		log.V(5).Info("skip policy as direct changes to pods managed by workload controllers are not allowed", "policy", ctx.Policy.GetName())
		return resp
	}

	ctx.JSONContext.Checkpoint()
	defer ctx.JSONContext.Restore()

	for _, rule := range ctx.Policy.Spec.Rules {
		if !rule.HasValidate() {
			continue
		}

		log = log.WithValues("rule", rule.Name)

		if !matches(log, rule, ctx) {
			continue
		}

		ctx.JSONContext.Restore()
		if err := LoadContext(log, rule.Context, ctx.ResourceCache, ctx); err != nil {
			log.Error(err, "failed to load context")
			continue
		}

		log.V(3).Info("matched validate rule")

		// operate on the copy of the conditions, as we perform variable substitution
		preconditionsCopy, err := copyConditions(rule.AnyAllConditions)
		if err != nil {
			log.V(2).Info("wrongfully configured data", "reason", err.Error())
			continue
		}
		// evaluate pre-conditions
		// - handle variable substitutions
		if !variables.EvaluateConditions(log, ctx.JSONContext, preconditionsCopy) {
			log.V(4).Info("resource fails the preconditions")
			continue
		}

		if rule.Validation.Pattern != nil || rule.Validation.AnyPattern != nil {
			ruleResponse := validateResourceWithRule(log, ctx, rule)
			if ruleResponse != nil {
				if !common.IsConditionalAnchorError(ruleResponse.Message) {
					incrementAppliedCount(resp)
					resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResponse)
				}
			}
		} else if rule.Validation.Deny != nil {
			denyConditionsCopy, err := copyConditions(rule.Validation.Deny.AnyAllConditions)
			if err != nil {
				log.V(2).Info("wrongfully configured data", "reason", err.Error())
				continue
			}
			deny := variables.EvaluateConditions(log, ctx.JSONContext, denyConditionsCopy)
			ruleResp := response.RuleResponse{
				Name:    rule.Name,
				Type:    utils.Validation.String(),
				Message: rule.Validation.Message,
				Success: !deny,
			}

			incrementAppliedCount(resp)
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, ruleResp)
		}
	}

	return resp
}

func validateResourceWithRule(log logr.Logger, ctx *PolicyContext, rule kyverno.Rule) (resp *response.RuleResponse) {
	if reflect.DeepEqual(ctx.OldResource, unstructured.Unstructured{}) {
		resp := validatePatterns(log, ctx.JSONContext, ctx.NewResource, rule)
		return &resp
	}

	if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
		log.V(3).Info("skipping validation on deleted resource")
		return nil
	}

	oldResp := validatePatterns(log, ctx.JSONContext, ctx.OldResource, rule)
	newResp := validatePatterns(log, ctx.JSONContext, ctx.NewResource, rule)
	if isSameRuleResponse(oldResp, newResp) {
		log.V(3).Info("skipping modified resource as validation results have not changed")
		return nil
	}

	return &newResp
}

// matches checks if either the new or old resource satisfies the filter conditions defined in the rule
func matches(logger logr.Logger, rule kyverno.Rule, ctx *PolicyContext) bool {
	err := MatchesResourceDescription(ctx.NewResource, rule, ctx.AdmissionInfo, ctx.ExcludeGroupRole, ctx.NamespaceLabels)
	if err == nil {
		return true
	}

	if !reflect.DeepEqual(ctx.OldResource, unstructured.Unstructured{}) {
		err := MatchesResourceDescription(ctx.OldResource, rule, ctx.AdmissionInfo, ctx.ExcludeGroupRole, ctx.NamespaceLabels)
		if err == nil {
			return true
		}
	}

	logger.V(4).Info("resource does not match rule", "reason", err.Error())
	return false
}

func isSameRuleResponse(r1 response.RuleResponse, r2 response.RuleResponse) bool {
	if r1.Name != r2.Name {
		return false
	}

	if r1.Type != r2.Type {
		return false
	}

	if r1.Message != r2.Message {
		return false
	}

	if r1.Success != r2.Success {
		return false
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

	return resp
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
