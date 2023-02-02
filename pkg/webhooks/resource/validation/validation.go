package validation

import (
	"context"
	"fmt"
	"reflect"
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
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ValidationHandler interface {
	// HandleValidation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleValidation(context.Context, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, time.Time) (bool, string, []string)
}

func NewValidationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	engine engineapi.Engine,
	contextLoader engineapi.ContextLoaderFactory,
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
		contextLoader:    contextLoader,
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
	contextLoader    engineapi.ContextLoaderFactory
	pCache           policycache.Cache
	pcBuilder        webhookutils.PolicyContextBuilder
	eventGen         event.Interface
	admissionReports bool
	metrics          metrics.MetricsConfigManager
	cfg              config.Configuration
}

func (v *validationHandler) HandleValidation(
	ctx context.Context,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	admissionRequestTimestamp time.Time,
) (bool, string, []string) {
	resourceName := admissionutils.GetResourceName(request)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind)

	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource(), unstructured.Unstructured{}) {
		resource := policyContext.NewResource()
		deletionTimeStamp = resource.GetDeletionTimestamp()
	} else {
		resource := policyContext.OldResource()
		deletionTimeStamp = resource.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == admissionv1.Update {
		return true, "", nil
	}

	var engineResponses []*engineapi.EngineResponse
	failurePolicy := kyvernov1.Ignore
	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"pkg/webhooks/resource/validate",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy)
				if policy.GetSpec().GetFailurePolicy() == kyvernov1.Fail {
					failurePolicy = kyvernov1.Fail
				}

				engineResponse := v.engine.Validate(ctx, v.contextLoader, policyContext, v.cfg)
				if engineResponse.IsNil() {
					// we get an empty response if old and new resources created the same response
					// allow updates if resource update doesnt change the policy evaluation
					return
				}

				go webhookutils.RegisterPolicyResultsMetricValidation(ctx, logger, v.metrics, string(request.Operation), policyContext.Policy(), *engineResponse)
				go webhookutils.RegisterPolicyExecutionDurationMetricValidate(ctx, logger, v.metrics, string(request.Operation), policyContext.Policy(), *engineResponse)

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
	if deletionTimeStamp == nil {
		events := webhookutils.GenerateEvents(engineResponses, blocked)
		v.eventGen.Add(events...)
	}

	if blocked {
		logger.V(4).Info("admission request blocked")
		return false, webhookutils.GetBlockedMessages(engineResponses), nil
	}

	go v.handleAudit(ctx, policyContext.NewResource(), request, engineResponses...)

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings
}

func (v *validationHandler) buildAuditResponses(
	ctx context.Context,
	resource unstructured.Unstructured,
	request *admissionv1.AdmissionRequest,
) ([]*engineapi.EngineResponse, error) {
	policies := v.pCache.GetPolicies(policycache.ValidateAudit, request.Kind.Kind, request.Namespace)
	policyContext, err := v.pcBuilder.Build(request)
	if err != nil {
		return nil, err
	}
	var responses []*engineapi.EngineResponse
	for _, policy := range policies {
		tracing.ChildSpan(
			ctx,
			"pkg/webhooks/resource/validate",
			fmt.Sprintf("POLICY %s/%s", policy.GetNamespace(), policy.GetName()),
			func(ctx context.Context, span trace.Span) {
				policyContext := policyContext.WithPolicy(policy)
				responses = append(responses, v.engine.Validate(ctx, v.contextLoader, policyContext, v.cfg))
			},
		)
	}
	return responses, nil
}

func (v *validationHandler) handleAudit(
	ctx context.Context,
	resource unstructured.Unstructured,
	request *admissionv1.AdmissionRequest,
	engineResponses ...*engineapi.EngineResponse,
) {
	if !v.admissionReports {
		return
	}
	if request.DryRun != nil && *request.DryRun {
		return
	}
	// we don't need reports for deletions
	if request.Operation == admissionv1.Delete {
		return
	}
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Kind)) {
		return
	}
	tracing.Span(
		context.Background(),
		"",
		fmt.Sprintf("AUDIT %s %s", request.Operation, request.Kind),
		func(ctx context.Context, span trace.Span) {
			responses, err := v.buildAuditResponses(ctx, resource, request)
			if err != nil {
				v.log.Error(err, "failed to build audit responses")
			}
			events := webhookutils.GenerateEvents(responses, false)
			v.eventGen.Add(events...)
			responses = append(responses, engineResponses...)
			report := reportutils.BuildAdmissionReport(resource, request, request.Kind, responses...)
			// if it's not a creation, the resource already exists, we can set the owner
			if request.Operation != admissionv1.Create {
				gv := metav1.GroupVersion{Group: request.Kind.Group, Version: request.Kind.Version}
				controllerutils.SetOwner(report, gv.String(), request.Kind.Kind, resource.GetName(), resource.GetUID())
			}
			if len(report.GetResults()) > 0 {
				_, err = reportutils.CreateReport(ctx, report, v.kyvernoClient)
				if err != nil {
					v.log.Error(err, "failed to create report")
				}
			}
		},
		trace.WithLinks(trace.LinkFromContext(ctx)),
	)
}
