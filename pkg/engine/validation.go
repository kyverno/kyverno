package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/jmespath/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/pss"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	matched "github.com/kyverno/kyverno/pkg/utils/match"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/trace"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// Validate applies validation rules from policy on the resource
func Validate(ctx context.Context, rclient registryclient.Client, policyContext *PolicyContext) (resp *response.EngineResponse) {
	resp = &response.EngineResponse{}
	startTime := time.Now()

	logger := buildLogger(policyContext)
	logger.V(4).Info("start validate policy processing", "startTime", startTime)
	defer func() {
		buildResponse(policyContext, resp, startTime)
		logger.V(4).Info("finished policy processing", "processingTime", resp.PolicyResponse.ProcessingTime.String(), "validationRulesApplied", resp.PolicyResponse.RulesAppliedCount)
	}()

	resp = validateResource(ctx, logger, rclient, policyContext)
	return
}

func buildLogger(ctx *PolicyContext) logr.Logger {
	logger := logging.WithName("EngineValidate").WithValues("policy", ctx.policy.GetName())
	if reflect.DeepEqual(ctx.newResource, unstructured.Unstructured{}) {
		logger = logger.WithValues("kind", ctx.oldResource.GetKind(), "namespace", ctx.oldResource.GetNamespace(), "name", ctx.oldResource.GetName())
	} else {
		logger = logger.WithValues("kind", ctx.newResource.GetKind(), "namespace", ctx.newResource.GetNamespace(), "name", ctx.newResource.GetName())
	}

	return logger
}

func buildResponse(ctx *PolicyContext, resp *response.EngineResponse, startTime time.Time) {
	if reflect.DeepEqual(resp, response.EngineResponse{}) {
		return
	}

	if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
		// for delete requests patched resource will be oldResource since newResource is empty
		resource := ctx.newResource
		if reflect.DeepEqual(ctx.newResource, unstructured.Unstructured{}) {
			resource = ctx.oldResource
		}

		resp.PatchedResource = resource
	}

	resp.Policy = ctx.policy
	resp.PolicyResponse.Policy.Name = ctx.policy.GetName()
	resp.PolicyResponse.Policy.Namespace = ctx.policy.GetNamespace()
	resp.PolicyResponse.Resource.Name = resp.PatchedResource.GetName()
	resp.PolicyResponse.Resource.Namespace = resp.PatchedResource.GetNamespace()
	resp.PolicyResponse.Resource.Kind = resp.PatchedResource.GetKind()
	resp.PolicyResponse.Resource.APIVersion = resp.PatchedResource.GetAPIVersion()
	resp.PolicyResponse.ValidationFailureAction = ctx.policy.GetSpec().ValidationFailureAction

	for _, v := range ctx.policy.GetSpec().ValidationFailureActionOverrides {
		resp.PolicyResponse.ValidationFailureActionOverrides = append(resp.PolicyResponse.ValidationFailureActionOverrides, response.ValidationFailureActionOverride{Action: v.Action, Namespaces: v.Namespaces})
	}

	resp.PolicyResponse.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.PolicyExecutionTimestamp = startTime.Unix()
}

func validateResource(ctx context.Context, log logr.Logger, rclient registryclient.Client, enginectx *PolicyContext) *response.EngineResponse {
	resp := &response.EngineResponse{}

	enginectx.jsonContext.Checkpoint()
	defer enginectx.jsonContext.Restore()

	rules := autogen.ComputeRules(enginectx.policy)
	matchCount := 0
	applyRules := enginectx.policy.GetSpec().GetApplyRules()

	if enginectx.policy.IsNamespaced() {
		polNs := enginectx.policy.GetNamespace()
		if enginectx.newResource.Object != nil && (enginectx.newResource.GetNamespace() != polNs || enginectx.newResource.GetNamespace() == "") {
			return resp
		}
		if enginectx.oldResource.Object != nil && (enginectx.oldResource.GetNamespace() != polNs || enginectx.oldResource.GetNamespace() == "") {
			return resp
		}
	}

	for i := range rules {
		rule := &rules[i]
		log.V(3).Info("processing validation rule", "matchCount", matchCount, "applyRules", applyRules)
		enginectx.jsonContext.Reset()
		startTime := time.Now()
		ruleResp := tracing.ChildSpan1(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) *response.RuleResponse {
				hasValidate := rule.HasValidate()
				hasValidateImage := rule.HasImagesValidationChecks()
				hasYAMLSignatureVerify := rule.HasYAMLSignatureVerify()
				if !hasValidate && !hasValidateImage {
					return nil
				}
				log = log.WithValues("rule", rule.Name)
				kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
				subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(kindsInPolicy, enginectx)

				if !matches(log, rule, enginectx, subresourceGVKToAPIResource) {
					return nil
				}
				// check if there is a corresponding policy exception
				ruleResp := hasPolicyExceptions(enginectx, rule, subresourceGVKToAPIResource, log)
				if ruleResp != nil {
					return ruleResp
				}
				log.V(3).Info("processing validation rule", "matchCount", matchCount, "applyRules", applyRules)
				enginectx.jsonContext.Reset()
				if hasValidate && !hasYAMLSignatureVerify {
					return processValidationRule(ctx, log, rclient, enginectx, rule)
				} else if hasValidateImage {
					return processImageValidationRule(ctx, log, rclient, enginectx, rule)
				} else if hasYAMLSignatureVerify {
					return processYAMLValidationRule(log, enginectx, rule)
				}
				return nil
			},
		)
		if ruleResp != nil {
			addRuleResponse(log, resp, ruleResp, startTime)
			if applyRules == kyvernov1.ApplyOne && resp.PolicyResponse.RulesAppliedCount > 0 {
				break
			}
		}
	}

	return resp
}

func processValidationRule(ctx context.Context, log logr.Logger, rclient registryclient.Client, policyContext *PolicyContext, rule *kyvernov1.Rule) *response.RuleResponse {
	v := newValidator(log, rclient, policyContext, rule)
	return v.validate(ctx)
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
	policyContext    *PolicyContext
	rule             *kyvernov1.Rule
	contextEntries   []kyvernov1.ContextEntry
	anyAllConditions apiextensions.JSON
	pattern          apiextensions.JSON
	anyPattern       apiextensions.JSON
	deny             *kyvernov1.Deny
	podSecurity      *kyvernov1.PodSecurity
	forEach          []kyvernov1.ForEachValidation
	rclient          registryclient.Client
	nesting          int
}

func newValidator(log logr.Logger, rclient registryclient.Client, ctx *PolicyContext, rule *kyvernov1.Rule) *validator {
	ruleCopy := rule.DeepCopy()
	return &validator{
		log:              log,
		rule:             ruleCopy,
		policyContext:    ctx,
		rclient:          rclient,
		contextEntries:   ruleCopy.Context,
		anyAllConditions: ruleCopy.GetAnyAllConditions(),
		pattern:          ruleCopy.Validation.GetPattern(),
		anyPattern:       ruleCopy.Validation.GetAnyPattern(),
		deny:             ruleCopy.Validation.Deny,
		podSecurity:      ruleCopy.Validation.PodSecurity,
		forEach:          ruleCopy.Validation.ForEachValidation,
	}
}

func newForEachValidator(foreach kyvernov1.ForEachValidation, rclient registryclient.Client, nesting int, rule *kyvernov1.Rule, ctx *PolicyContext, log logr.Logger) (*validator, error) {
	ruleCopy := rule.DeepCopy()
	anyAllConditions, err := datautils.ToMap(foreach.AnyAllConditions)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ruleCopy.Validation.ForEachValidation.AnyAllConditions")
	}

	nestedForEach, err := api.DeserializeJSONArray[kyvernov1.ForEachValidation](foreach.ForEachValidation)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert ruleCopy.Validation.ForEachValidation.AnyAllConditions")
	}

	return &validator{
		log:              log,
		policyContext:    ctx,
		rule:             ruleCopy,
		rclient:          rclient,
		contextEntries:   foreach.Context,
		anyAllConditions: anyAllConditions,
		pattern:          foreach.GetPattern(),
		anyPattern:       foreach.GetAnyPattern(),
		deny:             foreach.Deny,
		forEach:          nestedForEach,
		nesting:          nesting,
	}, nil
}

func (v *validator) validate(ctx context.Context) *response.RuleResponse {
	if err := v.loadContext(ctx); err != nil {
		return ruleError(v.rule, response.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := checkPreconditions(v.log, v.policyContext, v.anyAllConditions)
	if err != nil {
		return ruleError(v.rule, response.Validation, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return ruleResponse(*v.rule, response.Validation, "preconditions not met", response.RuleStatusSkip)
	}

	if v.deny != nil {
		return v.validateDeny()
	}

	if v.pattern != nil || v.anyPattern != nil {
		if err = v.substitutePatterns(); err != nil {
			return ruleError(v.rule, response.Validation, "variable substitution failed", err)
		}

		ruleResponse := v.validateResourceWithRule()
		return ruleResponse
	}

	if v.podSecurity != nil {
		if !isDeleteRequest(v.policyContext) {
			ruleResponse := v.validatePodSecurity()
			return ruleResponse
		}
	}

	if v.forEach != nil {
		ruleResponse := v.validateForEach(ctx)
		return ruleResponse
	}

	v.log.V(2).Info("invalid validation rule: podSecurity, patterns, or deny expected")
	return nil
}

func (v *validator) validateForEach(ctx context.Context) *response.RuleResponse {
	applyCount := 0
	for _, foreach := range v.forEach {
		elements, err := evaluateList(foreach.List, (v.policyContext.JSONContext()))
		if err != nil {
			v.log.V(2).Info("failed to evaluate list", "list", foreach.List, "error", err.Error())
			continue
		}

		resp, count := v.validateElements(ctx, v.rclient, foreach, elements, foreach.ElementScope)
		if resp.Status != response.RuleStatusPass {
			return resp
		}

		applyCount += count
	}

	if applyCount == 0 {
		if v.forEach == nil {
			return nil
		}

		return ruleResponse(*v.rule, response.Validation, "rule skipped", response.RuleStatusSkip)
	}

	return ruleResponse(*v.rule, response.Validation, "rule passed", response.RuleStatusPass)
}

func (v *validator) validateElements(ctx context.Context, rclient registryclient.Client, foreach kyvernov1.ForEachValidation, elements []interface{}, elementScope *bool) (*response.RuleResponse, int) {
	v.policyContext.jsonContext.Checkpoint()
	defer v.policyContext.jsonContext.Restore()
	applyCount := 0

	for i, e := range elements {
		if e == nil {
			continue
		}

		// TODO - this needs to be refactored. The engine should not have a dependency to the CLI code
		store.SetForEachElement(i)

		v.policyContext.JSONContext().Reset()
		policyContext := v.policyContext.Copy()
		if err := addElementToContext(policyContext, e, i, v.nesting, elementScope); err != nil {
			v.log.Error(err, "failed to add element to context")
			return ruleError(v.rule, response.Validation, "failed to process foreach", err), applyCount
		}

		foreachValidator, err := newForEachValidator(foreach, rclient, v.nesting+1, v.rule, policyContext, v.log)
		if err != nil {
			v.log.Error(err, "failed to create foreach validator")
			return ruleError(v.rule, response.Validation, "failed to create foreach validator", err), applyCount
		}

		r := foreachValidator.validate(ctx)
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
				return ruleResponse(*v.rule, response.Validation, msg, r.Status), applyCount
			}
			msg := fmt.Sprintf("validation failure: %v", r.Message)
			return ruleResponse(*v.rule, response.Validation, msg, r.Status), applyCount
		}

		applyCount++
	}

	return ruleResponse(*v.rule, response.Validation, "", response.RuleStatusPass), applyCount
}

func addElementToContext(ctx *PolicyContext, e interface{}, elementIndex, nesting int, elementScope *bool) error {
	data, err := variables.DocumentToUntyped(e)
	if err != nil {
		return err
	}
	if err := ctx.JSONContext().AddElement(data, elementIndex, nesting); err != nil {
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
		ctx.element = u
	}
	return nil
}

func (v *validator) loadContext(ctx context.Context) error {
	if err := LoadContext(ctx, v.log, v.rclient, v.contextEntries, v.policyContext, v.rule.Name); err != nil {
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
	anyAllCond, err := variables.SubstituteAll(v.log, v.policyContext.jsonContext, anyAllCond)
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

	deny := variables.EvaluateConditions(v.log, v.policyContext.jsonContext, denyConditions)
	if deny {
		return ruleResponse(*v.rule, response.Validation, v.getDenyMessage(deny), response.RuleStatusFail)
	}

	return ruleResponse(*v.rule, response.Validation, v.getDenyMessage(deny), response.RuleStatusPass)
}

func (v *validator) getDenyMessage(deny bool) string {
	if !deny {
		return fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
	}
	msg := v.rule.Validation.Message
	if msg == "" {
		return fmt.Sprintf("validation error: rule %s failed", v.rule.Name)
	}
	raw, err := variables.SubstituteAll(v.log, v.policyContext.jsonContext, msg)
	if err != nil {
		return msg
	}
	switch typed := raw.(type) {
	case string:
		return typed
	default:
		return "the produced message didn't resolve to a string, check your policy definition."
	}
}

func getSpec(v *validator) (podSpec *corev1.PodSpec, metadata *metav1.ObjectMeta, err error) {
	kind := v.policyContext.newResource.GetKind()

	if kind == "DaemonSet" || kind == "Deployment" || kind == "Job" || kind == "StatefulSet" || kind == "ReplicaSet" || kind == "ReplicationController" {
		var deployment appsv1.Deployment

		resourceBytes, err := v.policyContext.newResource.MarshalJSON()
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

		resourceBytes, err := v.policyContext.newResource.MarshalJSON()
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

		resourceBytes, err := v.policyContext.newResource.MarshalJSON()
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
	podSecurityChecks := &response.PodSecurityChecks{
		Level:   v.podSecurity.Level,
		Version: v.podSecurity.Version,
		Checks:  pssChecks,
	}
	if allowed {
		msg := fmt.Sprintf("Validation rule '%s' passed.", v.rule.Name)
		rspn := ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusPass)
		rspn.PodSecurityChecks = podSecurityChecks
		return rspn
	} else {
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, v.rule.Name, v.podSecurity.Level, v.podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		rspn := ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusFail)
		rspn.PodSecurityChecks = podSecurityChecks
		return rspn
	}
}

func (v *validator) validateResourceWithRule() *response.RuleResponse {
	if !isEmptyUnstructured(&v.policyContext.element) {
		return v.validatePatterns(v.policyContext.element)
	}

	if isDeleteRequest(v.policyContext) {
		v.log.V(3).Info("skipping validation on deleted resource")
		return nil
	}

	resp := v.validatePatterns(v.policyContext.newResource)
	return resp
}

func isDeleteRequest(ctx *PolicyContext) bool {
	// if the OldResource is not empty, and the NewResource is empty, the request is a DELETE
	return isEmptyUnstructured(&ctx.newResource)
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
func matches(logger logr.Logger, rule *kyvernov1.Rule, ctx *PolicyContext, subresourceGVKToAPIResource map[string]*metav1.APIResource) bool {
	err := MatchesResourceDescription(subresourceGVKToAPIResource, ctx.newResource, *rule, ctx.admissionInfo, ctx.excludeGroupRole, ctx.namespaceLabels, "", ctx.subresource)
	if err == nil {
		return true
	}

	if !reflect.DeepEqual(ctx.OldResource, unstructured.Unstructured{}) {
		err := MatchesResourceDescription(subresourceGVKToAPIResource, ctx.oldResource, *rule, ctx.admissionInfo, ctx.excludeGroupRole, ctx.namespaceLabels, "", ctx.subresource)
		if err == nil {
			return true
		}
	}

	logger.V(5).Info("resource does not match rule", "reason", err.Error())
	return false
}

// validatePatterns validate pattern and anyPattern
func (v *validator) validatePatterns(resource unstructured.Unstructured) *response.RuleResponse {
	if v.pattern != nil {
		if err := validate.MatchPattern(v.log, resource.Object, v.pattern); err != nil {
			pe, ok := err.(*validate.PatternError)
			if ok {
				v.log.V(3).Info("validation error", "path", pe.Path, "error", err.Error())

				if pe.Skip {
					return ruleResponse(*v.rule, response.Validation, pe.Error(), response.RuleStatusSkip)
				}

				if pe.Path == "" {
					return ruleResponse(*v.rule, response.Validation, v.buildErrorMessage(err, ""), response.RuleStatusError)
				}

				return ruleResponse(*v.rule, response.Validation, v.buildErrorMessage(err, pe.Path), response.RuleStatusFail)
			}

			return ruleResponse(*v.rule, response.Validation, v.buildErrorMessage(err, pe.Path), response.RuleStatusError)
		}

		v.log.V(4).Info("successfully processed rule")
		msg := fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
		return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusPass)
	}

	if v.anyPattern != nil {
		var failedAnyPatternsErrors []error
		var skippedAnyPatternErrors []error
		var err error

		anyPatterns, err := deserializeAnyPattern(v.anyPattern)
		if err != nil {
			msg := fmt.Sprintf("failed to deserialize anyPattern, expected type array: %v", err)
			return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusError)
		}

		for idx, pattern := range anyPatterns {
			err := validate.MatchPattern(v.log, resource.Object, pattern)
			if err == nil {
				msg := fmt.Sprintf("validation rule '%s' anyPattern[%d] passed.", v.rule.Name, idx)
				return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusPass)
			}

			if pe, ok := err.(*validate.PatternError); ok {
				var patternErr error
				v.log.V(3).Info("validation rule failed", "anyPattern[%d]", idx, "path", pe.Path)

				if pe.Skip {
					patternErr = fmt.Errorf("rule %s[%d] skipped: %s", v.rule.Name, idx, err.Error())
					skippedAnyPatternErrors = append(skippedAnyPatternErrors, patternErr)
				} else {
					if pe.Path == "" {
						patternErr = fmt.Errorf("rule %s[%d] failed: %s", v.rule.Name, idx, err.Error())
					} else {
						patternErr = fmt.Errorf("rule %s[%d] failed at path %s", v.rule.Name, idx, pe.Path)
					}
					failedAnyPatternsErrors = append(failedAnyPatternsErrors, patternErr)
				}
			}
		}

		// Any Pattern validation errors
		if len(skippedAnyPatternErrors) > 0 && len(failedAnyPatternsErrors) == 0 {
			var errorStr []string
			for _, err := range skippedAnyPatternErrors {
				errorStr = append(errorStr, err.Error())
			}

			v.log.V(4).Info(fmt.Sprintf("Validation rule '%s' skipped. %s", v.rule.Name, errorStr))
			return ruleResponse(*v.rule, response.Validation, strings.Join(errorStr, " "), response.RuleStatusSkip)
		} else if len(failedAnyPatternsErrors) > 0 {
			var errorStr []string
			for _, err := range failedAnyPatternsErrors {
				errorStr = append(errorStr, err.Error())
			}

			v.log.V(4).Info(fmt.Sprintf("Validation rule '%s' failed. %s", v.rule.Name, errorStr))
			msg := buildAnyPatternErrorMessage(v.rule, errorStr)
			return ruleResponse(*v.rule, response.Validation, msg, response.RuleStatusFail)
		}
	}

	return ruleResponse(*v.rule, response.Validation, v.rule.Validation.Message, response.RuleStatusPass)
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

	msgRaw, sErr := variables.SubstituteAll(v.log, v.policyContext.jsonContext, v.rule.Validation.Message)
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
		i, err := variables.SubstituteAll(v.log, v.policyContext.jsonContext, v.pattern)
		if err != nil {
			return err
		}

		v.pattern = i.(apiextensions.JSON)
		return nil
	}

	if v.anyPattern != nil {
		i, err := variables.SubstituteAll(v.log, v.policyContext.jsonContext, v.anyPattern)
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

	i, err := variables.SubstituteAll(v.log, v.policyContext.jsonContext, v.deny)
	if err != nil {
		return err
	}

	v.deny = i.(*kyvernov1.Deny)
	return nil
}

// matchesException checks if an exception applies to the resource being admitted
func matchesException(
	policyContext *PolicyContext,
	rule *kyvernov1.Rule,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
) (*kyvernov2alpha1.PolicyException, error) {
	candidates, err := policyContext.FindExceptions(rule.Name)
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		err := matched.CheckMatchesResources(
			policyContext.newResource,
			candidate.Spec.Match,
			policyContext.namespaceLabels,
			subresourceGVKToAPIResource,
			policyContext.subresource,
			policyContext.admissionInfo,
			policyContext.excludeGroupRole,
		)
		// if there's no error it means a match
		if err == nil {
			return candidate, nil
		}
	}
	return nil, nil
}

// hasPolicyExceptions returns nil when there are no matching exceptions.
// A rule response is returned when an exception is matched, or there is an error.
func hasPolicyExceptions(ctx *PolicyContext, rule *kyvernov1.Rule, subresourceGVKToAPIResource map[string]*metav1.APIResource, log logr.Logger) *response.RuleResponse {
	// if matches, check if there is a corresponding policy exception
	exception, err := matchesException(ctx, rule, subresourceGVKToAPIResource)
	// if we found an exception
	if err == nil && exception != nil {
		key, err := cache.MetaNamespaceKeyFunc(exception)
		if err != nil {
			log.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
			return &response.RuleResponse{
				Name:    rule.Name,
				Message: "failed to find matched exception " + key,
				Status:  response.RuleStatusError,
			}
		}
		log.V(3).Info("policy rule skipped due to policy exception", "exception", key)
		return &response.RuleResponse{
			Name:    rule.Name,
			Message: "rule skipped due to policy exception " + key,
			Status:  response.RuleStatusSkip,
		}
	}
	return nil
}
