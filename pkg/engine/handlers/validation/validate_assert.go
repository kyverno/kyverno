package validation

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/jmespath-community/go-jmespath/pkg/binding"
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno-json/pkg/engine/assert"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/cache"
)

type lazyBinding struct {
	resolver func() (any, error)
}

func (b *lazyBinding) Value() (any, error) {
	return b.resolver()
}

func newLazyBinding(jsonContext enginectx.EvalInterface, name string) binding.Binding {
	return &lazyBinding{
		resolver: sync.OnceValues(func() (any, error) {
			return jsonContext.Query(name)
		}),
	}
}

type validateAssertHandler struct{}

func NewValidateAssertHandler() (handlers.Handler, error) {
	return validateAssertHandler{}, nil
}

func (h validateAssertHandler) Process(
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
	// load context
	jsonContext := policyContext.JSONContext()
	jsonContext.Checkpoint()
	defer jsonContext.Restore()
	if err := contextLoader(ctx, rule.Context, jsonContext); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			logger.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			logger.Error(err, "failed to load context")
		}
		return resource, handlers.WithResponses(
			engineapi.RuleError(rule.Name, engineapi.Validation, "failed to load context", err, rule.ReportProperties),
		)
	}
	// prepare bindings
	bindings := binding.NewBindings()
	for _, entry := range rule.Context {
		bindings = bindings.Register("$"+entry.Name, newLazyBinding(jsonContext, entry.Name))
	}
	// execute assertion
	gvk, subResource := policyContext.ResourceKind()
	payload := map[string]any{
		"object":             policyContext.NewResource().Object,
		"oldObject":          policyContext.OldResource().Object,
		"admissionInfo":      policyContext.AdmissionInfo(),
		"operation":          policyContext.Operation(),
		"namespaceLabels":    policyContext.NamespaceLabels(),
		"admissionOperation": policyContext.AdmissionOperation(),
		"requestResource":    policyContext.RequestResource(),
		"resourceKind": map[string]any{
			"group":       gvk.Group,
			"version":     gvk.Version,
			"kind":        gvk.Kind,
			"subResource": subResource,
		},
	}
	asserttion := assert.Parse(ctx, rule.Validation.Assert.Value)
	errs, err := assert.Assert(ctx, nil, asserttion, payload, bindings)
	if err != nil {
		return resource, handlers.WithError(rule, engineapi.Validation, "failed to apply assertion", err)
	}
	// compose a response
	if len(errs) != 0 {
		var action kyvernov1.ValidationFailureAction
		if rule.Validation.FailureAction != nil {
			action = *rule.Validation.FailureAction
		} else {
			action = policyContext.Policy().GetSpec().ValidationFailureAction
		}

		// process the old object for UPDATE admission requests in case of enforce policies
		if action.Enforce() {
			allowExisitingViolations := rule.HasValidateAllowExistingViolations()
			if engineutils.IsUpdateRequest(policyContext) && allowExisitingViolations {
				errs, err := validateOldObject(ctx, logger, policyContext, rule, payload, bindings)
				if err != nil {
					logger.V(4).Info("warning: failed to validate old object", "rule", rule.Name, "error", err.Error())
					return resource, handlers.WithSkip(rule, engineapi.Validation, "failed to validate old object")
				}

				logger.V(3).Info("old object verification", "errors", errs)
				if len(errs) != 0 {
					logger.V(2).Info("warning: skipping the rule evaluation as pre-existing violations are allowed", "rule", rule.Name)
					return resource, handlers.WithSkip(rule, engineapi.Validation, "skipping the rule evaluation as pre-existing violations are allowed")
				}
			}
		}

		var responses []*engineapi.RuleResponse
		for _, err := range errs {
			responses = append(responses, engineapi.RuleFail(rule.Name, engineapi.Validation, err.Error(), rule.ReportProperties))
		}
		return resource, handlers.WithResponses(responses...)
	}
	msg := fmt.Sprintf("Validation rule '%s' passed.", rule.Name)
	return resource, handlers.WithResponses(
		engineapi.RulePass(rule.Name, engineapi.Validation, msg, rule.ReportProperties),
	)
}

func validateOldObject(ctx context.Context, logger logr.Logger, policyContext engineapi.PolicyContext, rule kyvernov1.Rule, payload map[string]any, bindings binding.Bindings) (errs field.ErrorList, err error) {
	if policyContext.Operation() != kyvernov1.Update {
		return nil, nil
	}

	oldResource := policyContext.OldResource()

	if err := policyContext.SetOperation(kyvernov1.Create); err != nil { // simulates the condition when old object was "created"
		return nil, errors.Wrapf(err, "failed to set operation")
	}

	payload["object"] = policyContext.OldResource().Object
	payload["oldObject"] = nil
	payload["operation"] = kyvernov1.Create

	defer func() {
		if err = policyContext.SetOperation(kyvernov1.Update); err != nil {
			logger.Error(errors.Wrapf(err, "failed to reset operation"), "")
		}

		payload["object"] = policyContext.NewResource().Object
		payload["oldObject"] = policyContext.OldResource().Object
		payload["operation"] = kyvernov1.Update
	}()

	if ok := matchResource(logger, oldResource, rule, policyContext.NamespaceLabels(), policyContext.Policy().GetNamespace(), kyvernov1.Create, policyContext.JSONContext()); !ok {
		return
	}

	assertion := assert.Parse(ctx, rule.Validation.Assert.Value)
	errs, err = assert.Assert(ctx, nil, assertion, payload, bindings)

	return
}
