package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/handlers/mutation"
	"github.com/kyverno/kyverno/pkg/engine/handlers/validation"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type engine struct {
	configuration              config.Configuration
	client                     dclient.Interface
	rclient                    registryclient.Client
	engineContextLoaderFactory engineapi.EngineContextLoaderFactory
	exceptionSelector          engineapi.PolicyExceptionSelector
	validateResourceHandler    handlers.Handler
	validateManifestHandler    handlers.Handler
	validatePssHandler         handlers.Handler
	validateImageHandler       handlers.Handler
	mutateResourceHandler      handlers.Handler
	mutateExistingHandler      handlers.Handler
}

func NewEngine(
	configuration config.Configuration,
	client dclient.Interface,
	rclient registryclient.Client,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
) engineapi.Engine {
	engineContextLoaderFactory := func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.EngineContextLoader {
		loader := contextLoader(policy, rule)
		return func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
			return loader.Load(
				ctx,
				client,
				rclient,
				contextEntries,
				jsonContext,
			)
		}
	}
	return &engine{
		configuration:              configuration,
		client:                     client,
		rclient:                    rclient,
		engineContextLoaderFactory: engineContextLoaderFactory,
		exceptionSelector:          exceptionSelector,
		validateResourceHandler:    validation.NewValidateResourceHandler(),
		validateManifestHandler:    validation.NewValidateManifestHandler(client),
		validatePssHandler:         validation.NewValidatePssHandler(),
		validateImageHandler:       validation.NewValidateImageHandler(configuration),
		mutateResourceHandler:      mutation.NewMutateResourceHandler(),
		mutateExistingHandler:      mutation.NewMutateExistingHandler(client),
	}
}

func (e *engine) Validate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext, time.Now())
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.validate"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse := e.validate(ctx, logger, policyContext)
		response = response.WithPolicyResponse(policyResponse)
	}
	return response.Done(time.Now())
}

func (e *engine) Mutate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext, time.Now())
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.mutate"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse, patchedResource := e.mutate(ctx, logger, policyContext)
		response = response.
			WithPolicyResponse(policyResponse).
			WithPatchedResource(patchedResource)
	}
	return response.Done(time.Now())
}

func (e *engine) Generate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext, time.Now())
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.generate"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse := e.generateResponse(ctx, logger, policyContext)
		response = response.WithPolicyResponse(policyResponse)
	}
	return response.Done(time.Now())
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) (engineapi.EngineResponse, engineapi.ImageVerificationMetadata) {
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext, time.Now())
	ivm := engineapi.ImageVerificationMetadata{}
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.verify"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse, innerIvm := e.verifyAndPatchImages(ctx, logger, policyContext)
		response, ivm = response.WithPolicyResponse(policyResponse), innerIvm
	}
	return response.Done(time.Now()), ivm
}

func (e *engine) ApplyBackgroundChecks(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext, time.Now())
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.background"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse := e.applyBackgroundChecks(ctx, logger, policyContext)
		response = response.WithPolicyResponse(policyResponse)
	}
	return response.Done(time.Now())
}

func (e *engine) ContextLoader(
	policy kyvernov1.PolicyInterface,
	rule kyvernov1.Rule,
) engineapi.EngineContextLoader {
	return e.engineContextLoaderFactory(policy, rule)
}

// matches checks if either the new or old resource satisfies the filter conditions defined in the rule
func matches(
	rule kyvernov1.Rule,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
) error {
	gvk, subresource := policyContext.ResourceKind()
	err := engineutils.MatchesResourceDescription(
		resource,
		rule,
		policyContext.AdmissionInfo(),
		policyContext.NamespaceLabels(),
		policyContext.Policy().GetNamespace(),
		gvk,
		subresource,
		policyContext.Operation(),
	)
	if err == nil {
		return nil
	}
	oldResource := policyContext.OldResource()
	if oldResource.Object != nil {
		err := engineutils.MatchesResourceDescription(
			policyContext.OldResource(),
			rule,
			policyContext.AdmissionInfo(),
			policyContext.NamespaceLabels(),
			policyContext.Policy().GetNamespace(),
			gvk,
			subresource,
			policyContext.Operation(),
		)
		if err == nil {
			return nil
		}
	}
	return err
}

func (e *engine) invokeRuleHandler(
	ctx context.Context,
	logger logr.Logger,
	handler handlers.Handler,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	ruleType engineapi.RuleType,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	return tracing.ChildSpan2(
		ctx,
		"pkg/engine",
		fmt.Sprintf("RULE %s", rule.Name),
		func(ctx context.Context, span trace.Span) (unstructured.Unstructured, []engineapi.RuleResponse) {
			// check if resource and rule match
			if err := matches(rule, policyContext, resource); err != nil {
				logger.V(4).Info("rule not matched", "reason", err.Error())
				return resource, nil
			}
			// check if there's an exception
			if ruleResp := e.hasPolicyExceptions(logger, ruleType, policyContext, rule); ruleResp != nil {
				return resource, handlers.RuleResponses(ruleResp)
			}
			// process handler
			return handler.Process(ctx, logger, policyContext, resource, rule, e.ContextLoader(policyContext.Policy(), rule))
		},
	)
}
