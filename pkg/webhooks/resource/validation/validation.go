package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	HandleValidationEnforce(context.Context, handlers.AdmissionRequest, []kyvernov1.PolicyInterface, time.Time) (bool, string, []string)
	HandleValidationAudit(context.Context, handlers.AdmissionRequest)
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
}

func (v *validationHandler) HandleValidationEnforce(
	ctx context.Context,
	request handlers.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	admissionRequestTimestamp time.Time,
) (bool, string, []string) {
	resourceName := admissionutils.GetResourceName(request.AdmissionRequest)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind)

	if len(policies) == 0 {
		return true, "", nil
	}

	policyContext, err := v.buildPolicyContextFromAdmissionRequest(logger, request)
	if err != nil {
		return false, "failed create policy context", nil
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
					logger.V(2).Info("validation failed", "action", policy.GetSpec().ValidationFailureAction, "policy", policy.GetName(), "failed rules", engineResponse.GetFailedRules())
					return
				}

				if len(engineResponse.GetSuccessRules()) > 0 {
					logger.V(2).Info("validation passed", "policy", policy.GetName())
				}
			},
		)
	}

	blocked := webhookutils.BlockRequest(engineResponses, failurePolicy, logger)
	events := webhookutils.GenerateEvents(engineResponses, blocked)
	v.eventGen.Add(events...)

	if blocked {
		logger.V(4).Info("admission request blocked")
		return false, webhookutils.GetBlockedMessages(engineResponses), nil
	}

	go func() {
		if needsReports(request, policyContext.NewResource(), v.admissionReports) {
			if err := v.createReports(ctx, policyContext.NewResource(), request, engineResponses...); err != nil {
				v.log.Error(err, "failed to create report")
			}
		}
	}()

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings
}

func (v *validationHandler) HandleValidationAudit(
	ctx context.Context,
	request handlers.AdmissionRequest,
) {
	gvr := schema.GroupVersionResource(request.Resource)
	policies := v.pCache.GetPolicies(policycache.ValidateAudit, gvr, request.SubResource, request.Namespace)
	if len(policies) == 0 {
		return
	}

	policyContext, err := v.buildPolicyContextFromAdmissionRequest(v.log, request)
	if err != nil {
		v.log.Error(err, "failed to build policy context")
		return
	}

	needsReport := needsReports(request, policyContext.NewResource(), v.admissionReports)
	tracing.Span(
		context.Background(),
		"",
		fmt.Sprintf("AUDIT %s %s", request.Operation, request.Kind),
		func(ctx context.Context, span trace.Span) {
			responses, err := v.buildAuditResponses(ctx, policyContext, policies)
			if err != nil {
				v.log.Error(err, "failed to build audit responses")
			}
			events := webhookutils.GenerateEvents(responses, false)
			v.eventGen.Add(events...)
			if needsReport {
				if err := v.createReports(ctx, policyContext.NewResource(), request, responses...); err != nil {
					v.log.Error(err, "failed to create report")
				}
			}
		},
		trace.WithLinks(trace.LinkFromContext(ctx)),
	)
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

func (v *validationHandler) buildPolicyContextFromAdmissionRequest(logger logr.Logger, request handlers.AdmissionRequest) (*policycontext.PolicyContext, error) {
	policyContext, err := v.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return nil, err
	}
	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = engineutils.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, v.nsLister, logger)
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
		_, err := reportutils.CreateReport(ctx, report, v.kyvernoClient)
		if err != nil {
			return err
		}
	}
	return nil
}
