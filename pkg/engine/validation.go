package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
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
		buildResponse(policyContext, resp, startTime)
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

func buildResponse(ctx *PolicyContext, resp *response.EngineResponse, startTime time.Time) {
	if reflect.DeepEqual(resp, response.EngineResponse{}) {
		return
	}

	if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
		// for delete requests patched resource will be oldResource since newResource is empty
		var resource = ctx.NewResource
		if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
			resource = ctx.OldResource
		}

		resp.PatchedResource = resource
	}

	resp.PolicyResponse.Policy.Name = ctx.Policy.GetName()
	resp.PolicyResponse.Policy.Namespace = ctx.Policy.GetNamespace()
	resp.PolicyResponse.Resource.Name = resp.PatchedResource.GetName()
	resp.PolicyResponse.Resource.Namespace = resp.PatchedResource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resp.PatchedResource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resp.PatchedResource.GetAPIVersion()
	resp.PolicyResponse.ValidationFailureAction = ctx.Policy.Spec.ValidationFailureAction
	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.PolicyExecutionTimestamp = startTime.Unix()
}

func validateResource(log logr.Logger, ctx *PolicyContext) *response.EngineResponse {
	resp := &response.EngineResponse{}
	if ManagedPodResource(ctx.Policy, ctx.NewResource) {
		log.V(5).Info("skip validation of pods managed by workload controllers", "policy", ctx.Policy.GetName())
		return resp
	}

	ctx.JSONContext.Checkpoint()
	defer ctx.JSONContext.Restore()

	for i := range ctx.Policy.Spec.Rules {
		rule := &ctx.Policy.Spec.Rules[i]
		if !rule.HasValidate() {
			continue
		}

		log = log.WithValues("rule", rule.Name)
		if !matches(log, rule, ctx) {
			continue
		}

		log.V(3).Info("matched validate rule")
		ctx.JSONContext.Reset()
		startTime := time.Now()

		ruleResp := processValidationRule(log, ctx, rule)
		if ruleResp != nil {
			addRuleResponse(log, resp, ruleResp, startTime)
		}
	}

	return resp
}

func processValidationRule(log logr.Logger, ctx *PolicyContext, rule *kyverno.Rule) *response.RuleResponse {
	v := newValidator(log, ctx, rule)
	if rule.Validation.ForEachValidation != nil {
		return v.validateForEach()
	}

	return v.validate()
}

func addRuleResponse(log logr.Logger, resp *response.EngineResponse, ruleResp *response.RuleResponse, startTime time.Time) {
	ruleResp.RuleStats.ProcessingTime = time.Since(startTime)
	ruleResp.RuleStats.RuleExecutionTimestamp = startTime.Unix()
	log.V(4).Info("finished processing rule", "processingTime", ruleResp.RuleStats.ProcessingTime.String())

	if ruleResp.Status == response.RuleStatusPass || ruleResp.Status == response.RuleStatusFail {
		incrementAppliedCount(resp)
	} else if ruleResp.Status == response.RuleStatusError {
		incrementErrorCount(resp)
	}

	resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
}

type validator struct {
	log              logr.Logger
	ctx              *PolicyContext
	rule             *kyverno.Rule
	contextEntries   []kyverno.ContextEntry
	anyAllConditions apiextensions.JSON
	pattern          apiextensions.JSON
	anyPattern       apiextensions.JSON
	deny             *kyverno.Deny
}

func newValidator(log logr.Logger, ctx *PolicyContext, rule *kyverno.Rule) *validator {
	ruleCopy := rule.DeepCopy()
	return &validator{
		log:              log,
		rule:             ruleCopy,
		ctx:              ctx,
		contextEntries:   ruleCopy.Context,
		anyAllConditions: ruleCopy.AnyAllConditions,
		pattern:          ruleCopy.Validation.Pattern,
		anyPattern:       ruleCopy.Validation.AnyPattern,
		deny:             ruleCopy.Validation.Deny,
	}
}

func newForeachValidator(log logr.Logger, ctx *PolicyContext, rule *kyverno.Rule, foreachIndex int) *validator {
	ruleCopy := rule.DeepCopy()
	foreach := ruleCopy.Validation.ForEachValidation
	anyAllConditions, err := common.ToMap(foreach[foreachIndex].AnyAllConditions)
	if err != nil {
		log.Error(err, "failed to convert ruleCopy.Validation.ForEachValidation.AnyAllConditions")
	}
	return &validator{
		log:              log,
		ctx:              ctx,
		rule:             ruleCopy,
		contextEntries:   foreach[foreachIndex].Context,
		anyAllConditions: anyAllConditions,
		pattern:          foreach[foreachIndex].Pattern,
		anyPattern:       foreach[foreachIndex].AnyPattern,
		deny:             foreach[foreachIndex].Deny,
	}
}

func (v *validator) validate() *response.RuleResponse {
	if err := v.loadContext(); err != nil {
		return ruleError(v.rule, utils.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(v.log, v.ctx, v.anyAllConditions)
	if err != nil {
		return ruleError(v.rule, utils.Validation, "failed to evaluate preconditions", err)
	} else if !preconditionsPassed {
		return ruleResponse(v.rule, utils.Validation, "preconditions not met", response.RuleStatusSkip)
	}

	if v.pattern != nil || v.anyPattern != nil {
		if err = v.substitutePatterns(); err != nil {
			return ruleError(v.rule, utils.Validation, "variable substitution failed", err)
		}

		ruleResponse := v.validateResourceWithRule()
		return ruleResponse

	} else if v.deny != nil {
		ruleResponse := v.validateDeny()
		return ruleResponse
	}

	v.log.Info("invalid validation rule: either patterns or deny conditions are expected")
	return nil
}

func (v *validator) validateForEach() *response.RuleResponse {
	if err := v.loadContext(); err != nil {
		return ruleError(v.rule, utils.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(v.log, v.ctx, v.anyAllConditions)
	if err != nil {
		return ruleError(v.rule, utils.Validation, "failed to evaluate preconditions", err)
	} else if !preconditionsPassed {
		return ruleResponse(v.rule, utils.Validation, "preconditions not met", response.RuleStatusSkip)
	}

	foreachList := v.rule.Validation.ForEachValidation
	applyCount := 0
	if foreachList == nil {
		return nil
	}

	for foreachIndex, foreach := range foreachList {
		elements, err := evaluateList(foreach.List, v.ctx.JSONContext)
		if err != nil {
			v.log.Info("failed to evaluate list", "list", foreach.List, "error", err.Error())
			continue
		}

		v.ctx.JSONContext.Checkpoint()
		defer v.ctx.JSONContext.Restore()

		for _, e := range elements {
			v.ctx.JSONContext.Reset()

			ctx := v.ctx.Copy()
			if err := addElementToContext(ctx, e); err != nil {
				v.log.Error(err, "failed to add element to context")
				return ruleError(v.rule, utils.Validation, "failed to process foreach", err)
			}

			foreach := newForeachValidator(v.log, ctx, v.rule, foreachIndex)
			r := foreach.validate()
			if r == nil {
				v.log.Info("skipping rule due to empty result")
				continue
			} else if r.Status == response.RuleStatusSkip {
				v.log.Info("skipping rule as preconditions were not met")
				continue
			} else if r.Status != response.RuleStatusPass {
				msg := fmt.Sprintf("validation failed in foreach rule for %v", r.Message)
				return ruleResponse(v.rule, utils.Validation, msg, r.Status)
			}
			applyCount++
		}
	}

	if applyCount == 0 {
		return ruleResponse(v.rule, utils.Validation, "rule skipped", response.RuleStatusSkip)
	}

	return ruleResponse(v.rule, utils.Validation, "rule passed", response.RuleStatusPass)
}

func addElementToContext(ctx *PolicyContext, e interface{}) error {
	data, err := common.ToMap(e)
	if err != nil {
		return err
	}

	jsonData := map[string]interface{}{
		"element": data,
	}

	if err := ctx.JSONContext.AddJSONObject(jsonData); err != nil {
		return errors.Wrapf(err, "failed to add element (%v) to JSON context", e)
	}

	u := unstructured.Unstructured{}
	u.SetUnstructuredContent(data)
	ctx.Element = u

	return nil
}

func (v *validator) loadContext() error {
	if err := LoadContext(v.log, v.contextEntries, v.ctx.ResourceCache, v.ctx, v.rule.Name); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			v.log.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			v.log.Error(err, "failed to load context")
		}

		return err
	}

	return nil
}

func (v *validator) validateDeny() *response.RuleResponse {
	anyAllCond := v.deny.AnyAllConditions
	anyAllCond, err := variables.SubstituteAll(v.log, v.ctx.JSONContext, anyAllCond)
	if err != nil {
		return ruleError(v.rule, utils.Validation, "failed to substitute variables in deny conditions", err)
	}

	if err = v.substituteDeny(); err != nil {
		return ruleError(v.rule, utils.Validation, "failed to substitute variables in rule", err)
	}

	denyConditions, err := transformConditions(anyAllCond)
	if err != nil {
		return ruleError(v.rule, utils.Validation, "invalid deny conditions", err)
	}

	deny := variables.EvaluateConditions(v.log, v.ctx.JSONContext, denyConditions)
	if deny {
		return ruleResponse(v.rule, utils.Validation, v.getDenyMessage(deny), response.RuleStatusFail)
	}

	return ruleResponse(v.rule, utils.Validation, v.getDenyMessage(deny), response.RuleStatusPass)
}

func (v *validator) getDenyMessage(deny bool) string {
	if !deny {
		return fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
	}

	msg := v.rule.Validation.Message
	if msg == "" {
		return fmt.Sprintf("validation error: rule %s failed", v.rule.Name)
	}

	raw, err := variables.SubstituteAll(v.log, v.ctx.JSONContext, msg)
	if err != nil {
		return msg
	}

	return raw.(string)
}

func (v *validator) validateResourceWithRule() *response.RuleResponse {
	if !isEmptyUnstructured(&v.ctx.Element) {
		resp := v.validatePatterns(v.ctx.Element)
		return resp
	}

	// if the OldResource is empty, the request is a CREATE
	if isEmptyUnstructured(&v.ctx.OldResource) {
		resp := v.validatePatterns(v.ctx.NewResource)
		return resp
	}

	// if the OldResource is not empty, and the NewResource is empty, the request is a DELETE
	if isEmptyUnstructured(&v.ctx.NewResource) {
		v.log.V(3).Info("skipping validation on deleted resource")
		return nil
	}

	// if the OldResource is not empty, and the NewResource is not empty, the request is a MODIFY
	oldResp := v.validatePatterns(v.ctx.OldResource)
	newResp := v.validatePatterns(v.ctx.NewResource)
	if isSameRuleResponse(oldResp, newResp) {
		v.log.V(3).Info("skipping modified resource as validation results have not changed")
		return nil
	}

	return newResp
}

func isEmptyUnstructured(u *unstructured.Unstructured) bool {
	if u == nil {
		return true
	}

	if reflect.DeepEqual(*u, unstructured.Unstructured{}) {
		return true
	}

	return false
}

// matches checks if either the new or old resource satisfies the filter conditions defined in the rule
func matches(logger logr.Logger, rule *kyverno.Rule, ctx *PolicyContext) bool {
	err := MatchesResourceDescription(ctx.NewResource, *rule, ctx.AdmissionInfo, ctx.ExcludeGroupRole, ctx.NamespaceLabels, "")
	if err == nil {
		return true
	}

	if !reflect.DeepEqual(ctx.OldResource, unstructured.Unstructured{}) {
		err := MatchesResourceDescription(ctx.OldResource, *rule, ctx.AdmissionInfo, ctx.ExcludeGroupRole, ctx.NamespaceLabels, "")
		if err == nil {
			return true
		}
	}

	logger.V(4).Info("resource does not match rule", "reason", err.Error())
	return false
}

func isSameRuleResponse(r1 *response.RuleResponse, r2 *response.RuleResponse) bool {
	if r1.Name != r2.Name {
		return false
	}

	if r1.Type != r2.Type {
		return false
	}

	if r1.Message != r2.Message {
		return false
	}

	if r1.Status != r2.Status {
		return false
	}

	return true
}

// validatePatterns validate pattern and anyPattern
func (v *validator) validatePatterns(resource unstructured.Unstructured) *response.RuleResponse {
	if v.pattern != nil {
		if err := validate.MatchPattern(v.log, resource.Object, v.pattern); err != nil {
			pe, ok := err.(*validate.PatternError)
			if ok {
				v.log.V(3).Info("validation error", "path", pe.Path, "error", err.Error())

				if pe.Skip {
					return ruleResponse(v.rule, utils.Validation, pe.Error(), response.RuleStatusSkip)
				}

				if pe.Path == "" {
					return ruleResponse(v.rule, utils.Validation, v.buildErrorMessage(err, ""), response.RuleStatusError)
				}

				return ruleResponse(v.rule, utils.Validation, v.buildErrorMessage(err, pe.Path), response.RuleStatusFail)
			}

			return ruleResponse(v.rule, utils.Validation, v.buildErrorMessage(err, pe.Path), response.RuleStatusError)
		}

		v.log.V(4).Info("successfully processed rule")
		msg := fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
		return ruleResponse(v.rule, utils.Validation, msg, response.RuleStatusPass)
	}

	if v.anyPattern != nil {
		var failedAnyPatternsErrors []error
		var err error

		anyPatterns, err := deserializeAnyPattern(v.anyPattern)
		if err != nil {
			msg := fmt.Sprintf("failed to deserialize anyPattern, expected type array: %v", err)
			return ruleResponse(v.rule, utils.Validation, msg, response.RuleStatusError)
		}

		for idx, pattern := range anyPatterns {
			err := validate.MatchPattern(v.log, resource.Object, pattern)
			if err == nil {
				msg := fmt.Sprintf("validation rule '%s' anyPattern[%d] passed.", v.rule.Name, idx)
				return ruleResponse(v.rule, utils.Validation, msg, response.RuleStatusPass)
			}

			if pe, ok := err.(*validate.PatternError); ok {
				v.log.V(3).Info("validation rule failed", "anyPattern[%d]", idx, "path", pe.Path)
				if pe.Path == "" {
					patternErr := fmt.Errorf("Rule %s[%d] failed: %s.", v.rule.Name, idx, err.Error())
					failedAnyPatternsErrors = append(failedAnyPatternsErrors, patternErr)
				} else {
					patternErr := fmt.Errorf("Rule %s[%d] failed at path %s.", v.rule.Name, idx, pe.Path)
					failedAnyPatternsErrors = append(failedAnyPatternsErrors, patternErr)
				}
			}
		}

		// Any Pattern validation errors
		if len(failedAnyPatternsErrors) > 0 {
			var errorStr []string
			for _, err := range failedAnyPatternsErrors {
				errorStr = append(errorStr, err.Error())
			}

			v.log.V(4).Info(fmt.Sprintf("Validation rule '%s' failed. %s", v.rule.Name, errorStr))
			msg := buildAnyPatternErrorMessage(v.rule, errorStr)
			return ruleResponse(v.rule, utils.Validation, msg, response.RuleStatusFail)
		}
	}

	return ruleResponse(v.rule, utils.Validation, v.rule.Validation.Message, response.RuleStatusPass)
}

func deserializeAnyPattern(anyPattern apiextensions.JSON) ([]interface{}, error) {
	if anyPattern == nil {
		return nil, nil
	}

	ap, err := json.Marshal(anyPattern)
	if err != nil {
		return nil, err
	}

	var res []interface{}
	if err := json.Unmarshal(ap, &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (v *validator) buildErrorMessage(err error, path string) string {
	if v.rule.Validation.Message == "" {
		if path != "" {
			return fmt.Sprintf("validation error: rule %s failed at path %s", v.rule.Name, path)
		}

		return fmt.Sprintf("validation error: rule %s execution error: %s", v.rule.Name, err.Error())
	}

	msgRaw, sErr := variables.SubstituteAll(v.log, v.ctx.JSONContext, v.rule.Validation.Message)
	if sErr != nil {
		v.log.Info("failed to substitute variables in message: %v", sErr)
	}

	msg := msgRaw.(string)
	if !strings.HasSuffix(msg, ".") {
		msg = msg + "."
	}

	if path != "" {
		return fmt.Sprintf("validation error: %s Rule %s failed at path %s", msg, v.rule.Name, path)
	}

	return fmt.Sprintf("validation error: %s Rule %s execution error: %s", msg, v.rule.Name, err.Error())
}

func buildAnyPatternErrorMessage(rule *kyverno.Rule, errors []string) string {
	errStr := strings.Join(errors, " ")
	if rule.Validation.Message == "" {
		return fmt.Sprintf("validation error: %s", errStr)
	}

	if strings.HasSuffix(rule.Validation.Message, ".") {
		return fmt.Sprintf("validation error: %s %s", rule.Validation.Message, errStr)
	}

	return fmt.Sprintf("validation error: %s. %s", rule.Validation.Message, errStr)
}

func (v *validator) substitutePatterns() error {
	if v.pattern != nil {
		i, err := variables.SubstituteAll(v.log, v.ctx.JSONContext, v.pattern)
		if err != nil {
			return err
		}

		v.pattern = i.(apiextensions.JSON)
		return nil
	}

	if v.anyPattern != nil {
		i, err := variables.SubstituteAll(v.log, v.ctx.JSONContext, v.anyPattern)
		if err != nil {
			return err
		}

		v.anyPattern = i.(apiextensions.JSON)
		return nil
	}

	return nil
}

func (v *validator) substituteDeny() error {
	if v.deny == nil {
		return nil
	}

	i, err := variables.SubstituteAll(v.log, v.ctx.JSONContext, v.deny)
	if err != nil {
		return err
	}

	v.deny = i.(*kyverno.Deny)
	return nil
}
