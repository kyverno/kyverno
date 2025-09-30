package validation

import (
	"context"
	"fmt"
	"time"

	json_patch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.opentelemetry.io/otel/trace"
	"gomodules.xyz/jsonpatch/v2"
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

	// Apply image verification mutation for UPDATE operations
	if request.Operation == "UPDATE" {
		imageVerifyPolicies := v.getImageVerificationPolicies(policies)
		if len(imageVerifyPolicies) > 0 {
			patchedRequest, err := v.applyImageVerificationMutation(ctx, request, imageVerifyPolicies)
			if err != nil {
				logger.Error(err, "failed to apply image verify policies")
				return false, "", nil, nil
			}
			request = patchedRequest
		}
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
				if reportutils.IsNamespaceTerminationError(err) {
					// Log namespace termination errors at debug level as they are expected
					v.log.V(2).Info("skipping report creation due to namespace termination", "error", err.Error())
				} else {
					v.log.Error(err, "failed to create report")
				}
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
					if reportutils.IsNamespaceTerminationError(err) {
						// Log namespace termination errors at debug level as they are expected
						v.log.V(2).Info("skipping report creation due to namespace termination", "error", err.Error())
					} else {
						v.log.Error(err, "failed to create report")
					}
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

func (v *validationHandler) getImageVerificationPolicies(policies []kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var imageVerifyPolicies []kyvernov1.PolicyInterface
	for _, policy := range policies {
		if policy.GetSpec().HasVerifyImages() {
			imageVerifyPolicies = append(imageVerifyPolicies, policy)
		}
	}
	return imageVerifyPolicies
}

func (v *validationHandler) applyImageVerificationMutation(
	ctx context.Context,
	request handlers.AdmissionRequest,
	imageVerifyPolicies []kyvernov1.PolicyInterface,
) (handlers.AdmissionRequest, error) {
	policyContext, err := v.pcBuilder.Build(request.AdmissionRequest, request.Roles, request.ClusterRoles, request.GroupVersionKind)
	if err != nil {
		return request, err
	}

	var allPatches []jsonpatch.JsonPatchOperation
	for _, policy := range imageVerifyPolicies {
		policyContext := policyContext.WithPolicy(policy)

		engineResponse, ivm := v.engine.VerifyAndPatchImages(ctx, policyContext)
		if !engineResponse.IsEmpty() {
			allPatches = append(allPatches, engineResponse.GetPatches()...)
		}

		if !ivm.IsEmpty() {
			resource := policyContext.NewResource()
			hasAnnotations := len(resource.GetAnnotations()) > 0
			ivmPatches, err := ivm.Patches(hasAnnotations, v.log)
			if err != nil {
				return request, fmt.Errorf("failed to generate image verification patches: %v", err)
			}
			allPatches = append(allPatches, ivmPatches...)
		}
	}

	if len(allPatches) > 0 {
		patchedRequest, err := v.applyPatchesToRequest(request, allPatches)
		if err != nil {
			return request, fmt.Errorf("failed to apply patches to request: %v", err)
		}
		return patchedRequest, nil
	}

	return request, nil
}

func (v *validationHandler) applyPatchesToRequest(
	request handlers.AdmissionRequest,
	patches []jsonpatch.JsonPatchOperation,
) (handlers.AdmissionRequest, error) {
	patchBytes := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)

	decoded, err := json_patch.DecodePatch(patchBytes)
	if err != nil {
		return request, fmt.Errorf("failed to decode patch: %v", err)
	}

	options := &json_patch.ApplyOptions{
		SupportNegativeIndices:   true,
		AllowMissingPathOnRemove: true,
		EnsurePathExistsOnAdd:    true,
	}

	patchedBytes, err := decoded.ApplyWithOptions(request.Object.Raw, options)
	if err != nil {
		return request, fmt.Errorf("failed to apply patch: %v", err)
	}

	newRequest := request
	newRequest.Object.Raw = patchedBytes
	return newRequest, nil
}
