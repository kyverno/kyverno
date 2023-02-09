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
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/pss"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils/api"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"go.opentelemetry.io/otel/trace"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (e *engine) validate(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) *engineapi.EngineResponse {
	startTime := time.Now()
	logger.V(4).Info("start validate policy processing", "startTime", startTime)
	policyResponse := e.validateResource(ctx, logger, policyContext)
	defer logger.V(4).Info("finished policy processing", "processingTime", policyResponse.ProcessingTime.String(), "validationRulesApplied", policyResponse.RulesAppliedCount)
	engineResponse := &engineapi.EngineResponse{PolicyResponse: *policyResponse}
	return internal.BuildResponse(policyContext, engineResponse, startTime)
}

func (e *engine) validateResource(
	ctx context.Context,
	logger logr.Logger,
	enginectx engineapi.PolicyContext,
) *engineapi.PolicyResponse {
	resp := &engineapi.PolicyResponse{}

	enginectx.JSONContext().Checkpoint()
	defer enginectx.JSONContext().Restore()

	rules := autogen.ComputeRules(enginectx.Policy())
	matchCount := 0
	applyRules := enginectx.Policy().GetSpec().GetApplyRules()
	newResource := enginectx.NewResource()
	oldResource := enginectx.OldResource()

	if enginectx.Policy().IsNamespaced() {
		polNs := enginectx.Policy().GetNamespace()
		if enginectx.NewResource().Object != nil && (newResource.GetNamespace() != polNs || newResource.GetNamespace() == "") {
			return resp
		}
		if enginectx.OldResource().Object != nil && (oldResource.GetNamespace() != polNs || oldResource.GetNamespace() == "") {
			return resp
		}
	}

	for i := range rules {
		rule := &rules[i]
		logger := internal.LoggerWithRule(logger, rules[i])
		logger.V(3).Info("processing validation rule", "matchCount", matchCount)
		enginectx.JSONContext().Reset()
		startTime := time.Now()
		ruleResp := tracing.ChildSpan1(
			ctx,
			"pkg/engine",
			fmt.Sprintf("RULE %s", rule.Name),
			func(ctx context.Context, span trace.Span) *engineapi.RuleResponse {
				hasValidate := rule.HasValidate()
				hasValidateImage := rule.HasImagesValidationChecks()
				hasYAMLSignatureVerify := rule.HasYAMLSignatureVerify()
				if !hasValidate && !hasValidateImage {
					return nil
				}
				kindsInPolicy := append(rule.MatchResources.GetKinds(), rule.ExcludeResources.GetKinds()...)
				subresourceGVKToAPIResource := GetSubresourceGVKToAPIResourceMap(e.client, kindsInPolicy, enginectx)

				if !matches(logger, rule, enginectx, subresourceGVKToAPIResource, e.configuration) {
					return nil
				}
				// check if there is a corresponding policy exception
				ruleResp := hasPolicyExceptions(logger, engineapi.Validation, e.exceptionSelector, enginectx, rule, subresourceGVKToAPIResource, e.configuration)
				if ruleResp != nil {
					return ruleResp
				}
				enginectx.JSONContext().Reset()
				if hasValidate && !hasYAMLSignatureVerify {
					return e.processValidationRule(ctx, logger, enginectx, rule)
				} else if hasValidateImage {
					return e.processImageValidationRule(ctx, logger, enginectx, rule)
				} else if hasYAMLSignatureVerify {
					return processYAMLValidationRule(e.client, logger, enginectx, rule)
				}
				return nil
			},
		)
		if ruleResp != nil {
			internal.AddRuleResponse(resp, ruleResp, startTime)
			logger.V(4).Info("finished processing rule", "processingTime", ruleResp.ExecutionStats.ProcessingTime.String())
		}
		if applyRules == kyvernov1.ApplyOne && resp.RulesAppliedCount > 0 {
			break
		}
	}

	return resp
}

func (e *engine) processValidationRule(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	rule *kyvernov1.Rule,
) *engineapi.RuleResponse {
	v := newValidator(logger, e.ContextLoader(policyContext.Policy(), *rule), policyContext, rule)
	return v.validate(ctx)
}

type validator struct {
	log              logr.Logger
	policyContext    engineapi.PolicyContext
	rule             *kyvernov1.Rule
	contextEntries   []kyvernov1.ContextEntry
	anyAllConditions apiextensions.JSON
	pattern          apiextensions.JSON
	anyPattern       apiextensions.JSON
	deny             *kyvernov1.Deny
	podSecurity      *kyvernov1.PodSecurity
	forEach          []kyvernov1.ForEachValidation
	contextLoader    engineapi.EngineContextLoader
	nesting          int
}

func newValidator(log logr.Logger, contextLoader engineapi.EngineContextLoader, ctx engineapi.PolicyContext, rule *kyvernov1.Rule) *validator {
	ruleCopy := rule.DeepCopy()
	return &validator{
		log:              log,
		rule:             ruleCopy,
		policyContext:    ctx,
		contextLoader:    contextLoader,
		contextEntries:   ruleCopy.Context,
		anyAllConditions: ruleCopy.GetAnyAllConditions(),
		pattern:          ruleCopy.Validation.GetPattern(),
		anyPattern:       ruleCopy.Validation.GetAnyPattern(),
		deny:             ruleCopy.Validation.Deny,
		podSecurity:      ruleCopy.Validation.PodSecurity,
		forEach:          ruleCopy.Validation.ForEachValidation,
	}
}

func newForEachValidator(
	foreach kyvernov1.ForEachValidation,
	contextLoader engineapi.EngineContextLoader,
	nesting int,
	rule *kyvernov1.Rule,
	ctx engineapi.PolicyContext,
	log logr.Logger,
) (*validator, error) {
	ruleCopy := rule.DeepCopy()
	anyAllConditions, err := datautils.ToMap(foreach.AnyAllConditions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ruleCopy.Validation.ForEachValidation.AnyAllConditions: %w", err)
	}

	nestedForEach, err := api.DeserializeJSONArray[kyvernov1.ForEachValidation](foreach.ForEachValidation)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ruleCopy.Validation.ForEachValidation.AnyAllConditions: %w", err)
	}

	return &validator{
		log:              log,
		policyContext:    ctx,
		rule:             ruleCopy,
		contextLoader:    contextLoader,
		contextEntries:   foreach.Context,
		anyAllConditions: anyAllConditions,
		pattern:          foreach.GetPattern(),
		anyPattern:       foreach.GetAnyPattern(),
		deny:             foreach.Deny,
		forEach:          nestedForEach,
		nesting:          nesting,
	}, nil
}

func (v *validator) validate(ctx context.Context) *engineapi.RuleResponse {
	if err := v.loadContext(ctx); err != nil {
		return internal.RuleError(v.rule, engineapi.Validation, "failed to load context", err)
	}

	preconditionsPassed, err := internal.CheckPreconditions(v.log, v.policyContext, v.anyAllConditions)
	if err != nil {
		return internal.RuleError(v.rule, engineapi.Validation, "failed to evaluate preconditions", err)
	}

	if !preconditionsPassed {
		return internal.RuleSkip(v.rule, engineapi.Validation, "preconditions not met")
	}

	if v.deny != nil {
		return v.validateDeny()
	}

	if v.pattern != nil || v.anyPattern != nil {
		if err = v.substitutePatterns(); err != nil {
			return internal.RuleError(v.rule, engineapi.Validation, "variable substitution failed", err)
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

func (v *validator) validateForEach(ctx context.Context) *engineapi.RuleResponse {
	applyCount := 0
	for _, foreach := range v.forEach {
		elements, err := evaluateList(foreach.List, v.policyContext.JSONContext())
		if err != nil {
			v.log.V(2).Info("failed to evaluate list", "list", foreach.List, "error", err.Error())
			continue
		}
		resp, count := v.validateElements(ctx, foreach, elements, foreach.ElementScope)
		if resp.Status != engineapi.RuleStatusPass {
			return resp
		}
		applyCount += count
	}
	if applyCount == 0 {
		if v.forEach == nil {
			return nil
		}
		return internal.RuleSkip(v.rule, engineapi.Validation, "rule skipped")
	}
	return internal.RuleResponse(*v.rule, engineapi.Validation, "rule passed", engineapi.RuleStatusPass)
}

func (v *validator) validateElements(ctx context.Context, foreach kyvernov1.ForEachValidation, elements []interface{}, elementScope *bool) (*engineapi.RuleResponse, int) {
	v.policyContext.JSONContext().Checkpoint()
	defer v.policyContext.JSONContext().Restore()
	applyCount := 0

	for index, element := range elements {
		if element == nil {
			continue
		}

		v.policyContext.JSONContext().Reset()
		policyContext := v.policyContext.Copy()
		if err := addElementToContext(policyContext, element, index, v.nesting, elementScope); err != nil {
			v.log.Error(err, "failed to add element to context")
			return internal.RuleError(v.rule, engineapi.Validation, "failed to process foreach", err), applyCount
		}

		foreachValidator, err := newForEachValidator(foreach, v.contextLoader, v.nesting+1, v.rule, policyContext, v.log)
		if err != nil {
			v.log.Error(err, "failed to create foreach validator")
			return internal.RuleError(v.rule, engineapi.Validation, "failed to create foreach validator", err), applyCount
		}

		r := foreachValidator.validate(ctx)
		if r == nil {
			v.log.V(2).Info("skip rule due to empty result")
			continue
		} else if r.Status == engineapi.RuleStatusSkip {
			v.log.V(2).Info("skip rule", "reason", r.Message)
			continue
		} else if r.Status != engineapi.RuleStatusPass {
			if r.Status == engineapi.RuleStatusError {
				if index < len(elements)-1 {
					continue
				}
				msg := fmt.Sprintf("validation failure: %v", r.Message)
				return internal.RuleResponse(*v.rule, engineapi.Validation, msg, r.Status), applyCount
			}
			msg := fmt.Sprintf("validation failure: %v", r.Message)
			return internal.RuleResponse(*v.rule, engineapi.Validation, msg, r.Status), applyCount
		}

		applyCount++
	}

	return internal.RuleResponse(*v.rule, engineapi.Validation, "", engineapi.RuleStatusPass), applyCount
}

func addElementToContext(ctx engineapi.PolicyContext, element interface{}, index, nesting int, elementScope *bool) error {
	data, err := variables.DocumentToUntyped(element)
	if err != nil {
		return err
	}
	if err := ctx.JSONContext().AddElement(data, index, nesting); err != nil {
		return fmt.Errorf("failed to add element (%v) to JSON context: %w", element, err)
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
		ctx.SetElement(u)
	}
	return nil
}

func (v *validator) loadContext(ctx context.Context) error {
	if err := v.contextLoader(ctx, v.contextEntries, v.policyContext.JSONContext()); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			v.log.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			v.log.Error(err, "failed to load context")
		}

		return err
	}

	return nil
}

func (v *validator) validateDeny() *engineapi.RuleResponse {
	if deny, err := internal.CheckDenyPreconditions(v.log, v.policyContext, v.deny.GetAnyAllConditions()); err != nil {
		return internal.RuleError(v.rule, engineapi.Validation, "failed to check deny preconditions", err)
	} else {
		if deny {
			return internal.RuleResponse(*v.rule, engineapi.Validation, v.getDenyMessage(deny), engineapi.RuleStatusFail)
		}
		return internal.RuleResponse(*v.rule, engineapi.Validation, v.getDenyMessage(deny), engineapi.RuleStatusPass)
	}
}

func (v *validator) getDenyMessage(deny bool) string {
	if !deny {
		return fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
	}
	msg := v.rule.Validation.Message
	if msg == "" {
		return fmt.Sprintf("validation error: rule %s failed", v.rule.Name)
	}
	raw, err := variables.SubstituteAll(v.log, v.policyContext.JSONContext(), msg)
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
	newResource := v.policyContext.NewResource()
	kind := newResource.GetKind()

	if kind == "DaemonSet" || kind == "Deployment" || kind == "Job" || kind == "StatefulSet" || kind == "ReplicaSet" || kind == "ReplicationController" {
		var deployment appsv1.Deployment

		resourceBytes, err := newResource.MarshalJSON()
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

		resourceBytes, err := newResource.MarshalJSON()
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

		resourceBytes, err := newResource.MarshalJSON()
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
func (v *validator) validatePodSecurity() *engineapi.RuleResponse {
	// Marshal pod metadata and spec
	podSpec, metadata, err := getSpec(v)
	if err != nil {
		return internal.RuleError(v.rule, engineapi.Validation, "Error while getting new resource", err)
	}

	pod := &corev1.Pod{
		Spec:       *podSpec,
		ObjectMeta: *metadata,
	}
	allowed, pssChecks, err := pss.EvaluatePod(v.podSecurity, pod)
	if err != nil {
		return internal.RuleError(v.rule, engineapi.Validation, "failed to parse pod security api version", err)
	}
	podSecurityChecks := &engineapi.PodSecurityChecks{
		Level:   v.podSecurity.Level,
		Version: v.podSecurity.Version,
		Checks:  pssChecks,
	}
	if allowed {
		msg := fmt.Sprintf("Validation rule '%s' passed.", v.rule.Name)
		rspn := internal.RuleResponse(*v.rule, engineapi.Validation, msg, engineapi.RuleStatusPass)
		rspn.PodSecurityChecks = podSecurityChecks
		return rspn
	} else {
		msg := fmt.Sprintf(`Validation rule '%s' failed. It violates PodSecurity "%s:%s": %s`, v.rule.Name, v.podSecurity.Level, v.podSecurity.Version, pss.FormatChecksPrint(pssChecks))
		rspn := internal.RuleResponse(*v.rule, engineapi.Validation, msg, engineapi.RuleStatusFail)
		rspn.PodSecurityChecks = podSecurityChecks
		return rspn
	}
}

func (v *validator) validateResourceWithRule() *engineapi.RuleResponse {
	element := v.policyContext.Element()
	if !isEmptyUnstructured(&element) {
		return v.validatePatterns(element)
	}
	if isDeleteRequest(v.policyContext) {
		v.log.V(3).Info("skipping validation on deleted resource")
		return nil
	}
	resp := v.validatePatterns(v.policyContext.NewResource())
	return resp
}

func isDeleteRequest(ctx engineapi.PolicyContext) bool {
	newResource := ctx.NewResource()
	// if the OldResource is not empty, and the NewResource is empty, the request is a DELETE
	return isEmptyUnstructured(&newResource)
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
func matches(
	logger logr.Logger,
	rule *kyvernov1.Rule,
	ctx engineapi.PolicyContext,
	subresourceGVKToAPIResource map[string]*metav1.APIResource,
	cfg config.Configuration,
) bool {
	err := MatchesResourceDescription(subresourceGVKToAPIResource, ctx.NewResource(), *rule, ctx.AdmissionInfo(), cfg.GetExcludeGroupRole(), ctx.NamespaceLabels(), "", ctx.SubResource())
	if err == nil {
		return true
	}

	if !reflect.DeepEqual(ctx.OldResource, unstructured.Unstructured{}) {
		err := MatchesResourceDescription(subresourceGVKToAPIResource, ctx.OldResource(), *rule, ctx.AdmissionInfo(), cfg.GetExcludeGroupRole(), ctx.NamespaceLabels(), "", ctx.SubResource())
		if err == nil {
			return true
		}
	}

	logger.V(5).Info("resource does not match rule", "reason", err.Error())
	return false
}

// validatePatterns validate pattern and anyPattern
func (v *validator) validatePatterns(resource unstructured.Unstructured) *engineapi.RuleResponse {
	if v.pattern != nil {
		if err := validate.MatchPattern(v.log, resource.Object, v.pattern); err != nil {
			pe, ok := err.(*validate.PatternError)
			if ok {
				v.log.V(3).Info("validation error", "path", pe.Path, "error", err.Error())

				if pe.Skip {
					return internal.RuleSkip(v.rule, engineapi.Validation, pe.Error())
				}

				if pe.Path == "" {
					return internal.RuleResponse(*v.rule, engineapi.Validation, v.buildErrorMessage(err, ""), engineapi.RuleStatusError)
				}

				return internal.RuleResponse(*v.rule, engineapi.Validation, v.buildErrorMessage(err, pe.Path), engineapi.RuleStatusFail)
			}

			return internal.RuleResponse(*v.rule, engineapi.Validation, v.buildErrorMessage(err, pe.Path), engineapi.RuleStatusError)
		}

		v.log.V(4).Info("successfully processed rule")
		msg := fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
		return internal.RuleResponse(*v.rule, engineapi.Validation, msg, engineapi.RuleStatusPass)
	}

	if v.anyPattern != nil {
		var failedAnyPatternsErrors []error
		var skippedAnyPatternErrors []error
		var err error

		anyPatterns, err := deserializeAnyPattern(v.anyPattern)
		if err != nil {
			msg := fmt.Sprintf("failed to deserialize anyPattern, expected type array: %v", err)
			return internal.RuleResponse(*v.rule, engineapi.Validation, msg, engineapi.RuleStatusError)
		}

		for idx, pattern := range anyPatterns {
			err := validate.MatchPattern(v.log, resource.Object, pattern)
			if err == nil {
				msg := fmt.Sprintf("validation rule '%s' anyPattern[%d] passed.", v.rule.Name, idx)
				return internal.RuleResponse(*v.rule, engineapi.Validation, msg, engineapi.RuleStatusPass)
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
			return internal.RuleSkip(v.rule, engineapi.Validation, strings.Join(errorStr, " "))
		} else if len(failedAnyPatternsErrors) > 0 {
			var errorStr []string
			for _, err := range failedAnyPatternsErrors {
				errorStr = append(errorStr, err.Error())
			}

			v.log.V(4).Info(fmt.Sprintf("Validation rule '%s' failed. %s", v.rule.Name, errorStr))
			msg := buildAnyPatternErrorMessage(v.rule, errorStr)
			return internal.RuleResponse(*v.rule, engineapi.Validation, msg, engineapi.RuleStatusFail)
		}
	}

	return internal.RuleResponse(*v.rule, engineapi.Validation, v.rule.Validation.Message, engineapi.RuleStatusPass)
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

	msgRaw, sErr := variables.SubstituteAll(v.log, v.policyContext.JSONContext(), v.rule.Validation.Message)
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
		i, err := variables.SubstituteAll(v.log, v.policyContext.JSONContext(), v.pattern)
		if err != nil {
			return err
		}
		v.pattern = i.(apiextensions.JSON)
		return nil
	}

	if v.anyPattern != nil {
		i, err := variables.SubstituteAll(v.log, v.policyContext.JSONContext(), v.anyPattern)
		if err != nil {
			return err
		}
		v.anyPattern = i.(apiextensions.JSON)
		return nil
	}

	return nil
}
