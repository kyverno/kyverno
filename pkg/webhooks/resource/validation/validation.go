package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type ValidationHandler interface {
	// HandleValidation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleValidationEnforce(context.Context, handlers.AdmissionRequest, []kyvernov1.PolicyInterface, []kyvernov1.PolicyInterface, time.Time) (bool, string, []string, []engineapi.EngineResponse)
	HandleValidationAudit(context.Context, handlers.AdmissionRequest) []engineapi.EngineResponse
}

func NewValidationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	engine engineapi.Engine,
	pCache policycache.Cache,
	pcBuilder webhookutils.PolicyContextBuilder,
	eventGen event.Interface,
	admissionReports bool,
	metrics metrics.MetricsConfigManager,
	cfg config.Configuration,
	nsLister corev1listers.NamespaceLister,
	reportConfig reportutils.ReportingConfiguration,
) ValidationHandler {
	return &validationHandler{
		log:              log,
		kyvernoClient:    kyvernoClient,
		engine:           engine,
		pCache:           pCache,
		pcBuilder:        pcBuilder,
		eventGen:         eventGen,
		admissionReports: admissionReports,
		metrics:          metrics,
		cfg:              cfg,
		nsLister:         nsLister,
		reportConfig:     reportConfig,
	}
}

type validationHandler struct {
	log              logr.Logger
	kyvernoClient    versioned.Interface
	engine           engineapi.Engine
	pCache           policycache.Cache
	pcBuilder        webhookutils.PolicyContextBuilder
	eventGen         event.Interface
	admissionReports bool
	metrics          metrics.MetricsConfigManager
	cfg              config.Configuration
	nsLister         corev1listers.NamespaceLister
	reportConfig     reportutils.ReportingConfiguration
}

func (v *validationHandler) HandleValidationEnforce(
	ctx context.Context,
	request handlers.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	auditWarnPolicies []kyvernov1.PolicyInterface,
	admissionRequestTimestamp time.Time,
) (bool, string, []string, []engineapi.EngineResponse) {
	resourceName := admissionutils.GetResourceName(request.AdmissionRequest)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind)

	if len(policies) == 0 && len(auditWarnPolicies) == 0 {
		return true, "", nil, nil
	}

	policyContext, err := v.buildPolicyContextFromAdmissionRequest(logger, request, policies)
	if err != nil {
		msg := fmt.Sprintf("failed to create policy context: %v", err)
		return false, msg, nil, nil
	}

	var engineResponses []engineapi.EngineResponse
	failurePolicy := kyvernov1.Ignore
	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"pkg/webhooks/resource/validate",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy)
				if policy.GetSpec().GetFailurePolicy(ctx) == kyvernov1.Fail {
					failurePolicy = kyvernov1.Fail
				}

				engineResponse := v.engine.Validate(ctx, policyContext)
				if engineResponse.IsNil() {
					// we get an empty response if old and new resources created the same response
					// allow updates if resource update doesn't change the policy evaluation
					return
				}

				engineResponses = append(engineResponses, engineResponse)
				if !engineResponse.IsSuccessful() {
					logger.V(2).Info("validation failed", "action", "Enforce", "policy", policy.GetName(), "failed rules", engineResponse.GetFailedRules())
					return
				}

				if len(engineResponse.GetSuccessRules()) > 0 {
					logger.V(2).Info("validation passed", "policy", policy.GetName())
				}
			},
		)
	}

	var auditWarnEngineResponses []engineapi.EngineResponse
	for _, policy := range auditWarnPolicies {
		tracing.ChildSpan(
			ctx,
			"pkg/webhooks/resource/validate",
			fmt.Sprintf("AUDIT WARN POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy)

				engineResponse := v.engine.Validate(ctx, policyContext)
				if engineResponse.IsNil() {
					// we get an empty response if old and new resources created the same response
					// allow updates if resource update doesn't change the policy evaluation
					return
				}

				auditWarnEngineResponses = append(auditWarnEngineResponses, engineResponse)
			},
		)
	}

	blocked := webhookutils.BlockRequest(engineResponses, failurePolicy, logger)

	if blocked {
		logger.V(4).Info("admission request blocked")
		return false, webhookutils.GetBlockedMessages(engineResponses), nil, engineResponses
	}

	go func() {
		if NeedsReports(request, policyContext.NewResource(), v.admissionReports, v.reportConfig) {
			if err := v.createReports(context.TODO(), policyContext.NewResource(), request, engineResponses...); err != nil {
				v.log.Error(err, "failed to create report")
			}
		}
	}()

	engineResponses = append(engineResponses, auditWarnEngineResponses...)
	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings, engineResponses
}

func (v *validationHandler) HandleValidationAudit(
	ctx context.Context,
	request handlers.AdmissionRequest,
) []engineapi.EngineResponse {
	gvr := schema.GroupVersionResource(request.Resource)
	policies := v.pCache.GetPolicies(policycache.ValidateAudit, gvr, request.SubResource, request.Namespace)
	if len(policies) == 0 {
		return nil
	}

	policyContext, err := v.buildPolicyContextFromAdmissionRequest(v.log, request, policies)
	if err != nil {
		v.log.Error(err, "failed to build policy context")
		return nil
	}

	var responses []engineapi.EngineResponse
	needsReport := NeedsReports(request, policyContext.NewResource(), v.admissionReports, v.reportConfig)
	tracing.Span(
		context.Background(),
		"",
		fmt.Sprintf("AUDIT %s %s", request.Operation, request.Kind),
		func(ctx context.Context, span trace.Span) {
			responses, err = v.buildAuditResponses(ctx, policyContext, policies)
			if err != nil {
				v.log.Error(err, "failed to build audit responses")
			}
			if needsReport {
				if err := v.createReports(ctx, policyContext.NewResource(), request, responses...); err != nil {
					v.log.Error(err, "failed to create report")
				}
			}
		},
		trace.WithLinks(trace.LinkFromContext(ctx)),
	)
	return responses
}

func (v *validationHandler) buildAuditResponses(
	ctx context.Context,
	policyContext *policycontext.PolicyContext,
	policies []kyvernov1.PolicyInterface,
) ([]engineapi.EngineResponse, error) {
	var responses []engineapi.EngineResponse
	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"pkg/webhooks/resource/validate",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy)
				response := v.engine.Validate(ctx, policyContext)
				responses = append(responses, response)
			},
		)
	}
	return responses, nil
}

func (v *validationHandler) buildPolicyContextFromAdmissionRequest(logger logr.Logger, request handlers.AdmissionRequest, policies []kyvernov1.PolicyInterface) (*policycontext.PolicyContext, error) {
	policyContext, err := v.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return nil, err
	}
	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels, err = engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, v.nsLister, policies, logger)
		if err != nil {
			return nil, err
		}
	}
	policyContext = policyContext.WithNamespaceLabels(namespaceLabels)
	return policyContext, nil
}

func (v *validationHandler) createReports(
	ctx context.Context,
	resource unstructured.Unstructured,
	request handlers.AdmissionRequest,
	engineResponses ...engineapi.EngineResponse,
) error {
	report := reportutils.BuildAdmissionReport(resource, request.AdmissionRequest, engineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
			// no need to set up open reports enabled here. create report is for an admission report (ephemeral)
			_, err := reportutils.CreateEphemeralReport(ctx, report, v.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
