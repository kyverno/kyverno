package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type validateImageVerificationHandler struct{}

func NewValidateImageVerificationHandler() (handlers.Handler, error) {
	return validateImageVerificationHandler{}, nil
}

func (h validateImageVerificationHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
	exceptions []*kyvernov2beta1.PolicyException,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	// check if there is a policy exception matches the incoming resource
	exception := engineutils.MatchesException(exceptions, policyContext, logger)
	if exception != nil {
		key, err := cache.MetaNamespaceKeyFunc(exception)
		if err != nil {
			logger.Error(err, "failed to compute policy exception key", "namespace", exception.GetNamespace(), "name", exception.GetName())
			return resource, handlers.WithError(rule, engineapi.Validation, "failed to compute exception key", err)
		} else {
			logger.V(3).Info("policy rule skipped due to policy exception", "exception", key)
			return resource, handlers.WithResponses(
				engineapi.RuleSkip(rule.Name, engineapi.Validation, "rule skipped due to policy exception "+key).WithException(exception),
			)
		}
	}
	v := newImageVerifyValidator(logger, contextLoader, policyContext, rule)
	return resource, handlers.WithResponses(v.validate(ctx))
}

type imageVerifyValidator struct {
	log            logr.Logger
	policyContext  engineapi.PolicyContext
	rule           kyvernov1.Rule
	contextEntries []kyvernov1.ContextEntry
	contextLoader  engineapi.EngineContextLoader
}

func newImageVerifyValidator(log logr.Logger, contextLoader engineapi.EngineContextLoader, ctx engineapi.PolicyContext, rule kyvernov1.Rule) *imageVerifyValidator {
	return &imageVerifyValidator{
		log:           log,
		rule:          rule,
		policyContext: ctx,
		contextLoader: contextLoader,
	}
}

func (v *imageVerifyValidator) validate(ctx context.Context) *engineapi.RuleResponse {
	if err := v.loadContext(ctx); err != nil {
		return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to load context", err)
	}

	for _, verifyImage := range v.rule.VerifyImages {
		if verifyImage.Validation.Deny != nil {
			return v.validateDeny(verifyImage.Validation.Deny)
		}
	}

	v.log.V(2).Info("invalid validation rule: podSecurity, cel, patterns, or deny expected")
	return nil
}

func (v *imageVerifyValidator) loadContext(ctx context.Context) error {
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

func (v *imageVerifyValidator) validateDeny(d *kyvernov1.Deny) *engineapi.RuleResponse {
	if deny, msg, err := internal.CheckDenyPreconditions(v.log, v.policyContext.JSONContext(), d.GetAnyAllConditions()); err != nil {
		return engineapi.RuleError(v.rule.Name, engineapi.Validation, "failed to check deny conditions", err)
	} else {
		if deny {
			return engineapi.RuleFail(v.rule.Name, engineapi.Validation, v.getDenyMessage(deny, msg))
		}
		return engineapi.RulePass(v.rule.Name, engineapi.Validation, v.getDenyMessage(deny, msg))
	}
}

func (v *imageVerifyValidator) getDenyMessage(deny bool, msg string) string {
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
