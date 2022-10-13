package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/pss"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Validate applies validation rules from policy on the resource
func Validate(policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	startTime := time.Now()

	logger := buildLogger(policyContext)
	logger.V(4).Info("start validate policy processing", "startTime", startTime)
	defer func() {
		buildResponse(policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "validationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	resp = validateResource(logger, policyContext)
	return
}

func buildLogger(ctx *PolicyContext) logr.Logger {
	logger := logging.WithName("EngineValidate").WithValues("policy", ctx.Policy.GetName())
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
		resource := ctx.NewResource
		if reflect.DeepEqual(ctx.NewResource, unstructured.Unstructured{}) {
			resource = ctx.OldResource
		}

		resp.PatchedResource = resource
	}

	resp.Policy = ctx.Policy
	resp.PolicyResponse.Policy.Name = ctx.Policy.GetName()
	resp.PolicyResponse.Policy.Namespace = ctx.Policy.GetNamespace()
	resp.PolicyResponse.Resource.Name = resp.PatchedResource.GetName()
	resp.PolicyResponse.Resource.Namespace = resp.PatchedResource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resp.PatchedResource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resp.PatchedResource.GetAPIVersion()
	resp.PolicyResponse.ValidationFailureAction = ctx.Policy.GetSpec().GetValidationFailureAction()

	for _, v := range ctx.Policy.GetSpec().ValidationFailureActionOverrides {
		resp.PolicyResponse.ValidationFailureActionOverrides = append(resp.PolicyResponse.ValidationFailureActionOverrides, response.ValidationFailureActionOverride{Action: v.Action, Namespaces: v.Namespaces})
	}

	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.PolicyExecutionTimestamp = startTime.Unix()
}

func validateResource(log logr.Logger, ctx *PolicyContext) *response.EngineResponse {
	resp := &response.EngineResponse{}

	ctx.JSONContext.Checkpoint()
	defer ctx.JSONContext.Restore()

	rules := autogen.ComputeRules(ctx.Policy)
	matchCount := 0
	applyRules := ctx.Policy.GetSpec().GetApplyRules()

	if ctx.Policy.IsNamespaced() {
		polNs := ctx.Policy.GetNamespace()
		if ctx.NewResource.Object != nil && (ctx.NewResource.GetNamespace() != polNs || ctx.NewResource.GetNamespace() == "") {
			return resp
		}
		if ctx.OldResource.Object != nil && (ctx.OldResource.GetNamespace() != polNs || ctx.OldResource.GetNamespace() == "") {
			return resp
		}
	}

	for i := range rules {
		rule := &rules[i]
		hasValidate := rule.HasValidate()
		hasValidateImage := rule.HasImagesValidationChecks()
		hasYAMLSignatureVerify := rule.HasYAMLSignatureVerify()
		if !hasValidate && !hasValidateImage {
			continue
		}

		log = log.WithValues("rule", rule.Name)
		if !matches(log, rule, ctx) {
			continue
		}

		log.V(3).Info("processing validation rule", "matchCount", matchCount, "applyRules", applyRules)
		ctx.JSONContext.Reset()
		startTime := time.Now()

		var ruleResp *response.RuleResponse
		if hasValidate && !hasYAMLSignatureVerify {
			ruleResp = processValidationRule(log, ctx, rule)
		} else if hasValidateImage {
			ruleResp = processImageValidationRule(log, ctx, rule)
		} else if hasYAMLSignatureVerify {
			ruleResp = processYAMLValidationRule(log, ctx, rule)
		}

		if ruleResp != nil {
			addRuleResponse(log, resp, ruleResp, startTime)
			if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.RulesAppliedCount > 0 {
				break
			}
		}
	}

	return resp
}

func validateOldObject(log logr.Logger, ctx *PolicyContext, rule *kyvernov1.Rule) (*response.RuleResponse, error) {
	ctxCopy := ctx.Copy()
	ctxCopy.NewResource = *ctxCopy.OldResource.DeepCopy()
	ctxCopy.OldResource = unstructured.Unstructured{}

	if err := context.ReplaceResource(ctxCopy.JSONContext, ctxCopy.NewResource.Object); err != nil {
		return nil, errors.Wrapf(err, "failed to replace object in the JSON context")
	}

	if err := context.ReplaceOldResource(ctxCopy.JSONContext, ctxCopy.OldResource.Object); err != nil {
		return nil, errors.Wrapf(err, "failed to replace old object in the JSON context")
	}

	return processValidationRule(log, ctxCopy, rule), nil
}

func processValidationRule(log logr.Logger, ctx *PolicyContext, rule *kyvernov1.Rule) *response.RuleResponse {
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
	rule             *kyvernov1.Rule
	contextEntries   []kyvernov1.ContextEntry
	anyAllConditions apiextensions.JSON
	pattern          apiextensions.JSON
	anyPattern       apiextensions.JSON
	deny             *kyvernov1.Deny
	podSecurity      *kyvernov1.PodSecurity
}

func newValidator(log logr.Logger, ctx *PolicyContext, rule *kyvernov1.Rule) *validator {
	ruleCopy := rule.DeepCopy()
	return &validator{
		log:              log,
		rule:             ruleCopy,
		ctx:              ctx,
		contextEntries:   ruleCopy.Context,
		anyAllConditions: ruleCopy.GetAnyAllConditions(),
		pattern:          ruleCopy.Validation.GetPattern(),
		anyPattern:       ruleCopy.Validation.GetAnyPattern(),
		deny:             ruleCopy.Validation.Deny,
		podSecurity:      ruleCopy.Validation.PodSecurity,
	}
}

func newForeachValidator(foreach kyvernov1.ForEachValidation, rule *kyvernov1.Rule, ctx *PolicyContext, log logr.Logger) *validator {
	ruleCopy := rule.DeepCopy()
	anyAllConditions, err := utils.ToMap(foreach.AnyAllConditions)
	if err != nil {
		log.Error(err, "failed to convert ruleCopy.Validation.ForEachValidation.AnyAllConditions")
	}

	return &validator{
		log:              log,
		ctx:              ctx,
		rule:             ruleCopy,
		contextEntries:   foreach.Context,
		anyAllConditions: anyAllConditions,
		pattern:          foreach.GetPattern(),
		anyPattern:       foreach.GetAnyPattern(),
		deny:             foreach.Deny,
	}
}

func (v *validator) validate() *response.RuleResponse {
	if err := v.loadContext(); err != nil {
		return ruleError(v.rule, response.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(v.log, v.ctx, v.anyAllConditions)
	if err != nil {
		return ruleError(v.rule, response.Validation, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return ruleResponse(*v.rule, response.Validation, "preconditions not met", response.RuleStatusSkip, nil)
	}

	if v.deny != nil {
		return v.validateDeny()
	}

	if v.pattern != nil || v.anyPattern != nil {
		if err = v.substitutePatterns(); err != nil {
			return ruleError(v.rule, response.Validation, "variable substitution failed", err)
		}

		ruleResponse := v.validateResourceWithRule()
		if isUpdateRequest(v.ctx) {
			priorResp, err := validateOldObject(v.log, v.ctx, v.rule)
			if err != nil {
				return ruleError(v.rule, response.Validation, "failed to validate old object", err)
			}

			if isSameRuleResponse(ruleResponse, priorResp) {
				v.log.V(3).Info("skipping modified resource as validation results have not changed")
				return nil
			}
		}

		return ruleResponse
	}

	if v.podSecurity != nil {
		if !isDeleteRequest(v.ctx) {
			ruleResponse := v.validatePodSecurity()
			return ruleResponse
		}
	}

	v.log.V(2).Info("invalid validation rule: podSecurity, patterns, or deny expected")
	return nil
}

func (v *validator) validateForEach() *response.RuleResponse {
	if err := v.loadContext(); err != nil {
		return ruleError(v.rule, response.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(v.log, v.ctx, v.anyAllConditions)
	if err != nil {
		return ruleError(v.rule, response.Validation, "failed to evaluate preconditions", err)
	} else if !preconditionsPassed {
		return ruleResponse(*v.rule, response.Validation, "preconditions not met", response.RuleStatusSkip, nil)
	}

	foreachList := v.rule.Validation.ForEachValidation
	applyCount := 0
	if foreachList == nil {
		return nil
	}

	for _, foreach := range foreachList {
		elements, err := evaluateList(foreach.List, v.ctx.JSONContext)
		if err != nil {
			v.log.V(2).Info("failed to evaluate list", "list", foreach.List, "error", err.Error())
			continue
		}

		resp, count := v.validateElements(foreach, elements, foreach.ElementScope)
		if resp.Status != response.RuleStatusPass {
			return resp
		}

		applyCount += count
	}

	if applyCount == 0 {
		return ruleResponse(*v.rule, response.Validation, "rule skipped", response.RuleStatusSkip, nil)
	}

	return ruleResponse(*v.rule, response.Validation, "rule passed", response.RuleStatusPass, nil)
}

func (v *validator) validateElements(foreach kyvernov1.ForEachValidation, elements []interface{}, elementScope *bool) (*response.RuleResponse, int) {
	v.ctx.JSONContext.Checkpoint()
	defer v.ctx.JSONContext.Restore()
	applyCount := 0

	for i, e := range elements {
		store.SetForeachElement(i)
		v.ctx.JSONContext.Reset()

		ctx := v.ctx.Copy()
		if err := addElementToContext(ctx, e, i, elementScope); err != nil {
			v.log.Error(err, "failed to add element to context")
			return ruleError(v.rule, response.Validation, "failed to process foreach", err), applyCount
		}

		foreachValidator := newForeachValidator(foreach, v.rule, ctx, v.log)
		r := foreachValidator.validate()
		if r == nil {
			v.log.V(2).Info("skip rule due to empty result")
			continue
		} else if r.Status == response.RuleStatusSkip {
			v.log.V(2).Info("skip rule", "reason", r.Message)
			continue
		} else if r.Status != response.RuleStatusPass {
			if r.Status == response.RuleStatusError {
				if i < len(elements)-1 {
					continue
				}
				msg := fmt.Sprintf("validation failure: %v", r.Message)
				return ruleResponse(*v.rule, response.Validation, msg, r.Status, nil), applyCount
			}
			msg := fmt.Sprintf("validation failure: %v", r.Message)
			return ruleResponse(*v.rule, response.Validation, msg, r.Status, nil), applyCount
		}

		applyCount++
	}

	return ruleResponse(*v.rule, response.Validation, "", response.RuleStatusPass, nil), applyCount
}

func addElementToContext(ctx *PolicyContext, e interface{}, elementIndex int, elementScope *bool) error {
	data, err := variables.DocumentToUntyped(e)
	if err != nil {
		return err
	}
	if err := ctx.JSONContext.AddElement(data, elementIndex); err != nil {
		return errors.Wrapf(err, "failed to add element (%v) to JSON context", e)
	}
	dataMap, ok := data.(map[string]interface{})
	// We set scoped to true by default if the data is a map
	// otherwise we do not do element scoped foreach unless the user
	// has explicitly set it to true
	scoped := ok

	// If the user has explicitly provided an element scope
	// we check if data is a map or not. In case it is not a map and the user
	// has set elementscoped to true, we throw an error.
	// Otherwise we set the value to what is specified by the user.
	if elementScope != nil {
		if *elementScope && !ok {
			return fmt.Errorf("cannot use elementScope=true foreach rules for elements that are not maps, expected type=map got type=%T", data)
		}
		scoped = *elementScope
	}

	if scoped {
		u := unstructured.Unstructured{}
		u.SetUnstructuredContent(dataMap)
		ctx.Element = u
	}
	return nil
}

func (v *validator) loadContext() error {
	if err := LoadContext(v.log, v.contextEntries, v.ctx, v.rule.Name); err != nil {
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
	anyAllCond := v.deny.GetAnyAllConditions()
	anyAllCond, err := variables.SubstituteAll(v.log, v.ctx.JSONContext, anyAllCond)
	if err != nil {
		return ruleError(v.rule, response.Validation, "failed to substitute variables in deny conditions", err)
	}

	if err = v.substituteDeny(); err != nil {
		return ruleError(v.rule, response.Validation, "failed to substitute variables in rule", err)
	}

	denyConditions, err := common.TransformConditions(anyAllCond)
	if err != nil {
		return ruleError(v.rule, response.Validation, "invalid deny conditions", err)
	}

	deny := variables.EvaluateConditions(v.log, v.ctx.JSONContext, denyConditions)
	if deny {
		return ruleResponse(*v.rule, response.Validation, v.getDenyMessage(deny), response.RuleStatusFail, nil)
	}

	return ruleResponse(*v.rule, response.Validation, v.getDenyMessage(deny), response.RuleStatusPass, nil)
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

func getSpec(v *validator) (podSpec *corev1.PodSpec, metadata *metav1.ObjectMeta, err error) {
	kind := v.ctx.NewResource.GetKind()

	if kind == "DaemonSet" || kind == "Deployment" || kind == "Job" || kind == "StatefulSet" {
		var deployment appsv1.Deployment

		resourceBytes, err := v.ctx.NewResource.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(resourceBytes, &deployment)
		if err != nil {
			return nil, nil, err
		}
		podSpec = &deployment.Spec.Template.Spec
		metadata = &deployment.Spec.Template.ObjectMeta
		return podSpec, metadata, nil
	} else if kind == "CronJob" {
		var cronJob batchv1.CronJob

		resourceBytes, err := v.ctx.NewResource.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(resourceBytes, &cronJob)
		if err != nil {
			return nil, nil, err
		}
		podSpec = &cronJob.Spec.JobTemplate.Spec.Template.Spec
		metadata = &cronJob.Spec.JobTemplate.ObjectMeta
	} else if kind == "Pod" {
		var pod corev1.Pod

		resourceBytes, err := v.ctx.NewResource.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}
		err = json.Unmarshal(resourceBytes, &pod)
		if err != nil {
			return nil, nil, err
		}
		podSpec = &pod.Spec
		metadata = &pod.ObjectMeta
		return podSpec, metadata, nil
	}

	if err != nil {
		return nil, nil, err
	}
	return podSpec, metadata, err
}

// Unstructured
func (v *validator) validatePodSecurity() *response.RuleResponse {
	// Marshal pod metadata and spec
	podSpec, metadata, err := getSpec(v)
	if err != nil {
		return ruleError(v.rule, response.Validation, "Error while getting new resource", err)
	}

	pod := &corev1.Pod{
		Spec:       *podSpec,
		ObjectMeta: *metadata,
	}
	allowed, pssChecks, err := pss.EvaluatePod(v.podSecurity, pod)
	if err != nil {
		return ruleError(v.rule, response.Validation, "failed to parse pod security api version", err)
	}
	if allowed {
		msg := fmt.Sprintf("Validation rule '%s' passed.", v.rule.Name)
		return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusPass, nil)
	} else {
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, v.rule.Name, v.podSecurity.Level, v.podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusFail, nil)
	}
}

func (v *validator) validateResourceWithRule() *response.RuleResponse {
	if !isEmptyUnstructured(&v.ctx.Element) {
		return v.validatePatterns(v.ctx.Element)
	}

	if isDeleteRequest(v.ctx) {
		v.log.V(3).Info("skipping validation on deleted resource")
		return nil
	}

	resp := v.validatePatterns(v.ctx.NewResource)
	return resp
}

func isDeleteRequest(ctx *PolicyContext) bool {
	// if the OldResource is not empty, and the NewResource is empty, the request is a DELETE
	return isEmptyUnstructured(&ctx.NewResource)
}

func isUpdateRequest(ctx *PolicyContext) bool {
	// is the OldObject and NewObject are available, the request is an UPDATE
	return !isEmptyUnstructured(&ctx.OldResource) && !isEmptyUnstructured(&ctx.NewResource)
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
func matches(logger logr.Logger, rule *kyvernov1.Rule, ctx *PolicyContext) bool {
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

	logger.V(5).Info("resource does not match rule", "reason", err.Error())
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
					return ruleResponse(*v.rule, response.Validation, pe.Error(), response.RuleStatusSkip, nil)
				}

				if pe.Path == "" {
					return ruleResponse(*v.rule, response.Validation, v.buildErrorMessage(err, ""), response.RuleStatusError, nil)
				}

				return ruleResponse(*v.rule, response.Validation, v.buildErrorMessage(err, pe.Path), response.RuleStatusFail, nil)
			}

			return ruleResponse(*v.rule, response.Validation, v.buildErrorMessage(err, pe.Path), response.RuleStatusError, nil)
		}

		v.log.V(4).Info("successfully processed rule")
		msg := fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
		return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusPass, nil)
	}

	if v.anyPattern != nil {
		var failedAnyPatternsErrors []error
		var err error

		anyPatterns, err := deserializeAnyPattern(v.anyPattern)
		if err != nil {
			msg := fmt.Sprintf("failed to deserialize anyPattern, expected type array: %v", err)
			return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusError, nil)
		}

		for idx, pattern := range anyPatterns {
			err := validate.MatchPattern(v.log, resource.Object, pattern)
			if err == nil {
				msg := fmt.Sprintf("validation rule '%s' anyPattern[%d] passed.", v.rule.Name, idx)
				return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusPass, nil)
			}

			if pe, ok := err.(*validate.PatternError); ok {
				v.log.V(3).Info("validation rule failed", "anyPattern[%d]", idx, "path", pe.Path)
				if pe.Path == "" {
					patternErr := fmt.Errorf("rule %s[%d] failed: %s", v.rule.Name, idx, err.Error())
					failedAnyPatternsErrors = append(failedAnyPatternsErrors, patternErr)
				} else {
					patternErr := fmt.Errorf("rule %s[%d] failed at path %s", v.rule.Name, idx, pe.Path)
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
			return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusFail, nil)
		}
	}

	return ruleResponse(*v.rule, response.Validation, v.rule.Validation.Message, response.RuleStatusPass, nil)
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
		v.log.V(2).Info("failed to substitute variables in message", "error", sErr)
		return fmt.Sprintf("validation error: variables substitution error in rule %s execution error: %s", v.rule.Name, err.Error())
	} else {
		msg := msgRaw.(string)
		if !strings.HasSuffix(msg, ".") {
			msg = msg + "."
		}
		if path != "" {
			return fmt.Sprintf("validation error: %s rule %s failed at path %s", msg, v.rule.Name, path)
		}
		return fmt.Sprintf("validation error: %s rule %s execution error: %s", msg, v.rule.Name, err.Error())
	}
}

func buildAnyPatternErrorMessage(rule *kyvernov1.Rule, errors []string) string {
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

	v.deny = i.(*kyvernov1.Deny)
	return nil
}
