package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type engine struct {
	configuration            config.Configuration
	metricsConfiguration     config.MetricsConfiguration
	jp                       jmespath.Interface
	client                   engineapi.Client
	rclientFactory           engineapi.RegistryClientFactory
	ivCache                  imageverifycache.Client
	contextLoader            engineapi.ContextLoaderFactory
	exceptionSelector        engineapi.PolicyExceptionSelector
	imageSignatureRepository string
	// metrics
	resultCounter     metric.Int64Counter
	durationHistogram metric.Float64Histogram
}

type handlerFactory = func() (handlers.Handler, error)

func NewEngine(
	configuration config.Configuration,
	metricsConfiguration config.MetricsConfiguration,
	jp jmespath.Interface,
	client engineapi.Client,
	rclientFactory engineapi.RegistryClientFactory,
	ivCache imageverifycache.Client,
	contextLoader engineapi.ContextLoaderFactory,
	exceptionSelector engineapi.PolicyExceptionSelector,
	imageSignatureRepository string,
) engineapi.Engine {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	resultCounter, err := meter.Int64Counter(
		"kyverno_policy_results",
		metric.WithDescription("can be used to track the results associated with the policies applied in the user's cluster, at the level from rule to policy to admission requests"),
	)
	if err != nil {
		logging.Error(err, "failed to register metric kyverno_policy_results")
	}
	durationHistogram, err := meter.Float64Histogram(
		"kyverno_policy_execution_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of the individual rules under Kyverno policies whenever they evaluate incoming resource requests"),
	)
	if err != nil {
		logging.Error(err, "failed to register metric kyverno_policy_execution_duration_seconds")
	}
	return &engine{
		configuration:            configuration,
		metricsConfiguration:     metricsConfiguration,
		jp:                       jp,
		client:                   client,
		rclientFactory:           rclientFactory,
		ivCache:                  ivCache,
		contextLoader:            contextLoader,
		exceptionSelector:        exceptionSelector,
		imageSignatureRepository: imageSignatureRepository,
		resultCounter:            resultCounter,
		durationHistogram:        durationHistogram,
	}
}

func (e *engine) Validate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext)
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.validate"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse := e.validate(ctx, logger, policyContext)
		response = response.WithPolicyResponse(policyResponse)
	}
	response = response.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
	e.reportMetrics(ctx, logger, policyContext.Operation(), policyContext.AdmissionOperation(), response)
	return response
}

func (e *engine) Mutate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext)
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.mutate"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse, patchedResource := e.mutate(ctx, logger, policyContext)
		response = response.
			WithPolicyResponse(policyResponse).
			WithPatchedResource(patchedResource)
	}
	response = response.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
	e.reportMetrics(ctx, logger, policyContext.Operation(), policyContext.AdmissionOperation(), response)
	return response
}

func (e *engine) Generate(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext)
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.generate"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse := e.generateResponse(ctx, logger, policyContext)
		response = response.WithPolicyResponse(policyResponse)
	}
	response = response.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
	e.reportMetrics(ctx, logger, policyContext.Operation(), policyContext.AdmissionOperation(), response)
	return response
}

func (e *engine) VerifyAndPatchImages(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) (engineapi.EngineResponse, engineapi.ImageVerificationMetadata) {
	startTime := time.Now()
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext)
	ivm := engineapi.ImageVerificationMetadata{}
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.verify"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse, patchedResource, innerIvm := e.verifyAndPatchImages(ctx, logger, policyContext)
		response, ivm = response.
			WithPolicyResponse(policyResponse).
			WithPatchedResource(patchedResource), innerIvm
	}
	response = response.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
	e.reportMetrics(ctx, logger, policyContext.Operation(), policyContext.AdmissionOperation(), response)
	return response, ivm
}

func (e *engine) ApplyBackgroundChecks(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
) engineapi.EngineResponse {
	startTime := time.Now()
	response := engineapi.NewEngineResponseFromPolicyContext(policyContext)
	logger := internal.LoggerWithPolicyContext(logging.WithName("engine.background"), policyContext)
	if internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		policyResponse := e.applyBackgroundChecks(ctx, logger, policyContext)
		response = response.WithPolicyResponse(policyResponse)
	}
	response = response.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
	e.reportMetrics(ctx, logger, policyContext.Operation(), policyContext.AdmissionOperation(), response)
	return response
}

func (e *engine) ContextLoader(
	policy kyvernov1.PolicyInterface,
	rule kyvernov1.Rule,
) engineapi.EngineContextLoader {
	loader := e.contextLoader(policy, rule)
	return func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return loader.Load(
			ctx,
			e.jp,
			e.client,
			e.rclientFactory,
			contextEntries,
			jsonContext,
		)
	}
}

// matches checks if either the new or old resource satisfies the filter conditions defined in the rule
func (e *engine) matches(
	rule kyvernov1.Rule,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
) error {
	if policyContext.AdmissionOperation() {
		request := policyContext.AdmissionInfo()
		if e.configuration.IsExcluded(request.AdmissionUserInfo.Username, request.AdmissionUserInfo.Groups, request.Roles, request.ClusterRoles) {
			return fmt.Errorf("excluded by configuration")
		}
	}
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
	if resource.Object == nil && oldResource.Object != nil {
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
	handlerFactory handlerFactory,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	ruleType engineapi.RuleType,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	return tracing.ChildSpan2(
		ctx,
		"pkg/engine",
		fmt.Sprintf("RULE %s", rule.Name),
		func(ctx context.Context, span trace.Span) (patchedResource unstructured.Unstructured, results []engineapi.RuleResponse) {
			// check if resource and rule match
			if err := e.matches(rule, policyContext, resource); err != nil {
				logger.V(4).Info("rule not matched", "reason", err.Error())
				return resource, nil
			}
			if handlerFactory == nil {
				return resource, handlers.WithError(rule, ruleType, "failed to instantiate handler", nil)
			} else if handler, err := handlerFactory(); err != nil {
				return resource, handlers.WithError(rule, ruleType, "failed to instantiate handler", err)
			} else if handler != nil {
				// check if there's an exception
				if ruleResp := e.hasPolicyExceptions(logger, ruleType, policyContext, rule); ruleResp != nil {
					return resource, handlers.WithResponses(ruleResp)
				}
				policyContext.JSONContext().Checkpoint()
				defer func() {
					policyContext.JSONContext().Restore()
					if patchedResource.Object != nil {
						if err := policyContext.JSONContext().AddResource(patchedResource.Object); err != nil {
							logger.Error(err, "failed to add resource in the json context")
						}
					}
				}()
				// load rule context
				contextLoader := e.ContextLoader(policyContext.Policy(), rule)
				if err := contextLoader(ctx, rule.Context, policyContext.JSONContext()); err != nil {
					if _, ok := err.(gojmespath.NotFoundError); ok {
						logger.V(3).Info("failed to load context", "reason", err.Error())
					} else {
						logger.Error(err, "failed to load context")
					}
					return resource, handlers.WithError(rule, ruleType, "failed to load context", err)
				}
				// check preconditions
				preconditionsPassed, msg, err := internal.CheckPreconditions(logger, policyContext.JSONContext(), rule.GetAnyAllConditions())
				if err != nil {
					return resource, handlers.WithError(rule, ruleType, "failed to evaluate preconditions", err)
				}
				if !preconditionsPassed {
					s := stringutils.JoinNonEmpty([]string{"preconditions not met", msg}, "; ")
					return resource, handlers.WithSkip(rule, ruleType, s)
				}
				// process handler
				return handler.Process(ctx, logger, policyContext, resource, rule, contextLoader)
			}
			return resource, nil
		},
	)
}
