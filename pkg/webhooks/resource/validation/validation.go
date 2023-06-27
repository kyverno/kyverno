package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ValidationHandler interface {
	// HandleValidation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleValidation(context.Context, handlers.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, time.Time) (bool, string, []string)
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
}

func (v *validationHandler) HandleValidation(
	ctx context.Context,
	request handlers.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
) (bool, string, []string) {
	resourceName := admissionutils.GetResourceName(request.AdmissionRequest)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind)

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
					// allow updates if resource update doesnt change the policy evaluation
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

	go v.handleAudit(ctx, policyContext.NewResource(), request, policyContext.NamespaceLabels(), engineResponses...)

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings
}

func (v *validationHandler) buildAuditResponses(
	ctx context.Context,
	resource unstructured.Unstructured,
	request handlers.AdmissionRequest,
	namespaceLabels map[string]string,
) ([]engineapi.EngineResponse, error) {
	gvr := schema.GroupVersionResource(request.Resource)
	policies := v.pCache.GetPolicies(policycache.ValidateAudit, gvr, request.SubResource, request.Namespace)
	policyContext, err := v.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return nil, err
	}
	var responses []engineapi.EngineResponse
	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"pkg/webhooks/resource/validate",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy).WithNamespaceLabels(namespaceLabels)
				response := v.engine.Validate(ctx, policyContext)
				responses = append(responses, response)
			},
		)
	}
	return responses, nil
}

func (v *validationHandler) handleAudit(
	ctx context.Context,
	resource unstructured.Unstructured,
	request handlers.AdmissionRequest,
	namespaceLabels map[string]string,
	engineResponses ...engineapi.EngineResponse,
) {
	createReport := v.admissionReports
	if admissionutils.IsDryRun(request.AdmissionRequest) {
		createReport = false
	}
	// we don't need reports for deletions
	if request.Operation == admissionv1.Delete {
		createReport = false
	}
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Kind)) {
		createReport = false
	}
	// if the underlying resource has no UID don't create a report
	if resource.GetUID() == "" {
		createReport = false
	}
	tracing.Span(
		context.Background(),
		"",
		fmt.Sprintf("AUDIT %s %s", request.Operation, request.Kind),
		func(ctx context.Context, span trace.Span) {
			responses, err := v.buildAuditResponses(ctx, resource, request, namespaceLabels)
			if err != nil {
				v.log.Error(err, "failed to build audit responses")
			}
			events := webhookutils.GenerateEvents(responses, false)
			v.eventGen.Add(events...)
			if createReport {
				responses = append(responses, engineResponses...)
				report := reportutils.BuildAdmissionReport(resource, request.AdmissionRequest, responses...)
				if len(report.GetResults()) > 0 {
					_, err = reportutils.CreateReport(ctx, report, v.kyvernoClient)
					if err != nil {
						v.log.Error(err, "failed to create report")
					}
				}
			}
		},
		trace.WithLinks(trace.LinkFromContext(ctx)),
	)
}
