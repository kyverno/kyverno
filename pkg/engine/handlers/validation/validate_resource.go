package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type validateResourceHandler struct{}

func NewValidateResourceHandler() (handlers.Handler, error) {
	return validateResourceHandler{}, nil
}

func (h validateResourceHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
	exceptions []*kyvernov2.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there are policy exceptions that match the incoming resource
	matchedExceptions := engineutils.MatchesException(exceptions, policyContext, logger)
	if len(matchedExceptions) > 0 {
		var keys []string
		for i, exception := range matchedExceptions {
			key, err := cache.MetaNamespaceKeyFunc(&matchedExceptions[i])
			if err != nil {
				logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
				return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
			}
			keys = append(keys, key)
		}

		logger.V(3).Info("policy rule is skipped due to policy exceptions", "exceptions", keys)
		return resource, handlers.WithResponses(
			engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule is skipped due to policy exceptions"+strings.Join(keys, ", "), rule.ReportProperties).WithExceptions(matchedExceptions),
		)
	}
	v := newValidator(logger, contextLoader, policyContext, rule)
	return resource, handlers.WithResponses(v.validate(ctx))
}

type validator struct {
	log              logr.Logger
	policyContext    engineapi.PolicyContext
	rule             kyvernov1.Rule
	contextEntries   []kyvernov1.ContextEntry
	anyAllConditions any
	pattern          apiextensions.JSON
	anyPattern       apiextensions.JSON
	deny             *kyvernov1.Deny
	forEach          []kyvernov1.ForEachValidation
	contextLoader    engineapi.EngineContextLoader
	nesting          int
}

func newValidator(log logr.Logger, contextLoader engineapi.EngineContextLoader, ctx engineapi.PolicyContext, rule kyvernov1.Rule) *validator {
	return &validator{
		log:              log,
		rule:             rule,
		policyContext:    ctx,
		contextLoader:    contextLoader,
		pattern:          rule.Validation.GetPattern(),
		anyPattern:       rule.Validation.GetAnyPattern(),
		deny:             rule.Validation.Deny,
		anyAllConditions: rule.GetAnyAllConditions(),
		forEach:          rule.Validation.ForEachValidation,
	}
}

func newForEachValidator(
	foreach kyvernov1.ForEachValidation,
	contextLoader engineapi.EngineContextLoader,
	nesting int,
	rule kyvernov1.Rule,
	ctx engineapi.PolicyContext,
	log logr.Logger,
) (*validator, error) {
	var loopItems []kyvernov1.ForEachValidation
	fev := foreach.GetForEachValidation()
	if len(fev) > 0 {
		loopItems = fev
	} else {
		loopItems = make([]kyvernov1.ForEachValidation, 0)
	}
	return &validator{
		log:              log,
		policyContext:    ctx,
		rule:             rule,
		contextLoader:    contextLoader,
		contextEntries:   foreach.Context,
		anyAllConditions: foreach.AnyAllConditions,
		pattern:          foreach.GetPattern(),
		anyPattern:       foreach.GetAnyPattern(),
		deny:             foreach.Deny,
		forEach:          loopItems,
		nesting:          nesting,
	}, nil
}

func (v *validator) validate(ctx context.Context) *engineapi.RuleResponse {
	if err := v.loadContext(ctx); err != nil {
		return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to load context", err, v.rule.ReportProperties)
	}
	preconditionsPassed, msg, err := internal.CheckPreconditions(v.log, v.policyContext.JSONContext(), v.anyAllConditions)
	if err != nil {
		return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to evaluate preconditions", err, v.rule.ReportProperties)
	}
	if !preconditionsPassed {
		s := stringutils.JoinNonEmpty([]string{"preconditions not met", msg}, "; ")
		return engineapi.RuleSkip(v.rule.Name, engineapi.Validation, s, v.rule.ReportProperties)
	}

	var ruleResponse *engineapi.RuleResponse
	if v.deny != nil {
		ruleResponse = v.validateDeny()
	} else if v.pattern != nil || v.anyPattern != nil {
		if err = v.substitutePatterns(); err != nil {
			return engineapi.RuleError(v.rule.Name, engineapi.Validation, "variable substitution failed", err, v.rule.ReportProperties)
		}

		ruleResponse = v.validateResourceWithRule()
	} else if v.forEach != nil {
		ruleResponse = v.validateForEach(ctx)
	} else {
		v.log.V(2).Info("invalid validation rule: podSecurity, cel, patterns, or deny expected")
	}

	var action kyvernov1.ValidationFailureAction
	if v.rule.Validation.FailureAction != nil {
		action = *v.rule.Validation.FailureAction
	} else {
		action = v.policyContext.Policy().GetSpec().ValidationFailureAction
	}

	// process the old object for UPDATE admission requests in case of enforce policies
	if action.Enforce() {
		allowExisitingViolations := v.rule.HasValidateAllowExistingViolations()
		if engineutils.IsUpdateRequest(v.policyContext) && allowExisitingViolations && v.nesting == 0 { // is update request and is the root level validate
			priorResp, err := v.validateOldObject(ctx)
			if err != nil {
				v.log.V(4).Info("warning: failed to validate old object", "rule", v.rule.Name, "error", err.Error())
				return engineapi.RuleSkip(v.rule.Name, engineapi.Validation, "failed to validate old object", ruleResponse.Properties())
			}

			// when an existing resource violates, and the updated resource also violates, then skip
			if (priorResp != nil && ruleResponse != nil) &&
				(ruleResponse.Status() == engineapi.RuleStatusFail && priorResp.Status() == engineapi.RuleStatusFail) {
				v.log.V(2).Info("warning: skipping the rule evaluation as pre-existing violations are allowed", "ruleResponse", ruleResponse, "priorResp", priorResp)
				return engineapi.RuleSkip(v.rule.Name, engineapi.Validation, "skipping the rule evaluation as pre-existing violations are allowed", v.rule.ReportProperties)
			}
		}
	}

	return ruleResponse
}

func (v *validator) validateOldObject(ctx context.Context) (resp *engineapi.RuleResponse, err error) {
	if v.policyContext.Operation() != kyvernov1.Update {
		return nil, errors.New("invalid operation")
	}

	newResource := v.policyContext.NewResource()
	oldResource := v.policyContext.OldResource()
	emptyResource := unstructured.Unstructured{}

	if err = v.policyContext.SetResources(emptyResource, oldResource); err != nil {
		return nil, errors.Wrapf(err, "failed to set resources")
	}

	if err = v.policyContext.SetOperation(kyvernov1.Create); err != nil { // simulates the condition when old object was "created"
		return nil, errors.Wrapf(err, "failed to set operation")
	}

	defer func() {
		if err = v.policyContext.SetResources(oldResource, newResource); err != nil {
			v.log.Error(errors.Wrapf(err, "failed to reset resources"), "")
		}

		if err = v.policyContext.SetOperation(kyvernov1.Update); err != nil {
			v.log.Error(errors.Wrapf(err, "failed to reset operation"), "")
		}
	}()

	if ok := matchResource(v.log, oldResource, v.rule, v.policyContext.NamespaceLabels(), v.policyContext.Policy().GetNamespace(), kyvernov1.Create, v.policyContext.JSONContext()); !ok {
		resp = engineapi.RuleSkip(v.rule.Name, engineapi.Validation, "resource not matched", nil)
		return
	}

	resp = v.validate(ctx)

	return
}

func (v *validator) validateForEach(ctx context.Context) *engineapi.RuleResponse {
	applyCount := 0
	for _, foreach := range v.forEach {
		elements, err := engineutils.EvaluateList(foreach.List, v.policyContext.JSONContext())
		if err != nil {
			v.log.V(2).Info("failed to evaluate list", "list", foreach.List, "error", err.Error())
			continue
		}
		resp, count := v.validateElements(ctx, foreach, elements, foreach.ElementScope)
		if resp.Status() != engineapi.RuleStatusPass {
			return resp
		}
		applyCount += count
	}
	if applyCount == 0 {
		return nil
	}
	return engineapi.RulePass(v.rule.Name, engineapi.Validation, "rule passed", v.rule.ReportProperties)
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
		if err := engineutils.AddElementToContext(policyContext, element, index, v.nesting, elementScope); err != nil {
			v.log.Error(err, "failed to add element to context")
			return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to process foreach", err, v.rule.ReportProperties), applyCount
		}

		foreachValidator, err := newForEachValidator(foreach, v.contextLoader, v.nesting+1, v.rule, policyContext, v.log)
		if err != nil {
			v.log.Error(err, "failed to create foreach validator")
			return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to create foreach validator", err, v.rule.ReportProperties), applyCount
		}

		r := foreachValidator.validate(ctx)
		if r == nil {
			v.log.V(2).Info("skip rule due to empty result")
			continue
		}
		status := r.Status()
		if status == engineapi.RuleStatusSkip {
			v.log.V(2).Info("skip rule", "reason", r.Message())
			continue
		} else if status != engineapi.RuleStatusPass {
			if status == engineapi.RuleStatusError {
				if index < len(elements)-1 {
					continue
				}
				msg := fmt.Sprintf("validation failure: %v", r.Message())
				return engineapi.NewRuleResponse(v.rule.Name, engineapi.Validation, msg, status, v.rule.ReportProperties), applyCount
			}
			msg := fmt.Sprintf("validation failure: %v", r.Message())
			return engineapi.NewRuleResponse(v.rule.Name, engineapi.Validation, msg, status, v.rule.ReportProperties), applyCount
		}

		applyCount++
	}

	return engineapi.RulePass(v.rule.Name, engineapi.Validation, "", v.rule.ReportProperties), applyCount
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
	if deny, msg, err := internal.CheckDenyPreconditions(v.log, v.policyContext.JSONContext(), v.deny.GetAnyAllConditions()); err != nil {
		return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to check deny conditions", err, v.rule.ReportProperties)
	} else {
		if deny {
			return engineapi.RuleFail(v.rule.Name, engineapi.Validation, v.getDenyMessage(deny, msg), v.rule.ReportProperties)
		}
		return engineapi.RulePass(v.rule.Name, engineapi.Validation, v.getDenyMessage(deny, msg), v.rule.ReportProperties)
	}
}

func (v *validator) getDenyMessage(deny bool, msg string) string {
	if !deny {
		return fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
	}

	if v.rule.Validation.Message == "" && msg == "" {
		return fmt.Sprintf("validation error: rule %s failed", v.rule.Name)
	}

	s := stringutils.JoinNonEmpty([]string{v.rule.Validation.Message, msg}, "; ")
	raw, err := variables.SubstituteAll(v.log, v.policyContext.JSONContext(), s)
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

func (v *validator) validateResourceWithRule() *engineapi.RuleResponse {
	element := v.policyContext.Element()
	if !engineutils.IsEmptyUnstructured(&element) {
		return v.validatePatterns(element)
	}
	if engineutils.IsDeleteRequest(v.policyContext) {
		v.log.V(3).Info("skipping validation on deleted resource")
		return nil
	}
	resp := v.validatePatterns(v.policyContext.NewResource())
	return resp
}

// validatePatterns validate pattern and anyPattern
func (v *validator) validatePatterns(resource unstructured.Unstructured) *engineapi.RuleResponse {
	if v.pattern != nil {
		if err := validate.MatchPattern(v.log, resource.Object, v.pattern); err != nil {
			pe, ok := err.(*validate.PatternError)
			if ok {
				v.log.V(3).Info("validation error", "path", pe.Path, "error", err.Error())

				if pe.Skip {
					return engineapi.RuleSkip(v.rule.Name, engineapi.Validation, pe.Error(), v.rule.ReportProperties)
				}

				if pe.Path == "" {
					return engineapi.RuleError(v.rule.Name, engineapi.Validation, v.buildErrorMessage(err, ""), nil, v.rule.ReportProperties)
				}

				return engineapi.RuleFail(v.rule.Name, engineapi.Validation, v.buildErrorMessage(err, pe.Path), v.rule.ReportProperties)
			}

			return engineapi.RuleError(v.rule.Name, engineapi.Validation, v.buildErrorMessage(err, ""), nil, v.rule.ReportProperties)
		}

		v.log.V(4).Info("successfully processed rule")
		msg := fmt.Sprintf("validation rule '%s' passed.", v.rule.Name)
		return engineapi.RulePass(v.rule.Name, engineapi.Validation, msg, v.rule.ReportProperties)
	}

	if v.anyPattern != nil {
		var failedAnyPatternsErrors []error
		var skippedAnyPatternErrors []error
		var err error

		anyPatterns, err := deserializeAnyPattern(v.anyPattern)
		if err != nil {
			return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to deserialize anyPattern, expected type array", err, v.rule.ReportProperties)
		}

		for idx, pattern := range anyPatterns {
			err := validate.MatchPattern(v.log, resource.Object, pattern)
			if err == nil {
				msg := fmt.Sprintf("validation rule '%s' anyPattern[%d] passed.", v.rule.Name, idx)
				return engineapi.RulePass(v.rule.Name, engineapi.Validation, msg, v.rule.ReportProperties)
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
			return engineapi.RuleSkip(v.rule.Name, engineapi.Validation, strings.Join(errorStr, " "), v.rule.ReportProperties)
		} else if len(failedAnyPatternsErrors) > 0 {
			var errorStr []string
			for _, err := range failedAnyPatternsErrors {
				errorStr = append(errorStr, err.Error())
			}

			v.log.V(4).Info(fmt.Sprintf("Validation rule '%s' failed. %s", v.rule.Name, errorStr))
			msg := v.buildAnyPatternErrorMessage(errorStr)
			return engineapi.RuleFail(v.rule.Name, engineapi.Validation, msg, v.rule.ReportProperties)
		}
	}

	return engineapi.RulePass(v.rule.Name, engineapi.Validation, v.rule.Validation.Message, v.rule.ReportProperties)
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

func (v *validator) buildAnyPatternErrorMessage(errors []string) string {
	errStr := strings.Join(errors, " ")
	if v.rule.Validation.Message == "" {
		return fmt.Sprintf("validation error: %s", errStr)
	}
	msgRaw, sErr := variables.SubstituteAll(v.log, v.policyContext.JSONContext(), v.rule.Validation.Message)
	if sErr != nil {
		v.log.V(2).Info("failed to substitute variables in message", "error", sErr)
		return fmt.Sprintf("validation error: variables substitution error in rule %s execution error: %s", v.rule.Name, errStr)
	} else {
		msg := msgRaw.(string)
		if strings.HasSuffix(msg, ".") {
			return fmt.Sprintf("validation error: %s %s", msg, errStr)
		}
		return fmt.Sprintf("validation error: %s. %s", msg, errStr)
	}
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
