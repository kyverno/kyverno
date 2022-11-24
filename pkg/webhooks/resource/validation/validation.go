package validation

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ValidationHandler interface {
	// HandleValidation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleValidation(*metrics.MetricsConfig, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, map[string]string, time.Time) (bool, string, []string)
}

func NewValidationHandler(
	log logr.Logger,
	kyvernoClient versioned.Interface,
	pCache policycache.Cache,
	pcBuilder webhookutils.PolicyContextBuilder,
	eventGen event.Interface,
	admissionReports bool,
) ValidationHandler {
	return &validationHandler{
		log:              log,
		kyvernoClient:    kyvernoClient,
		pCache:           pCache,
		pcBuilder:        pcBuilder,
		eventGen:         eventGen,
		admissionReports: admissionReports,
	}
}

type validationHandler struct {
	log              logr.Logger
	kyvernoClient    versioned.Interface
	pCache           policycache.Cache
	pcBuilder        webhookutils.PolicyContextBuilder
	eventGen         event.Interface
	admissionReports bool
}

func (v *validationHandler) HandleValidation(
	metricsConfig *metrics.MetricsConfig,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	namespaceLabels map[string]string,
	admissionRequestTimestamp time.Time,
) (bool, string, []string) {
	if len(policies) == 0 {
		// invoke handleAudit as we may have some policies in audit mode to consider
		go v.handleAudit(policyContext.NewResource, request, namespaceLabels)
		return true, "", nil
	}

	resourceName := admissionutils.GetResourceName(request)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource, unstructured.Unstructured{}) {
		deletionTimeStamp = policyContext.NewResource.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = policyContext.OldResource.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == admissionv1.Update {
		return true, "", nil
	}

	var engineResponses []*response.EngineResponse
	failurePolicy := kyvernov1.Ignore
	for _, policy := range policies {
		policyContext.Policy = policy
		policyContext.NamespaceLabels = namespaceLabels
		if policy.GetSpec().GetFailurePolicy() == kyvernov1.Fail {
			failurePolicy = kyvernov1.Fail
		}

		engineResponse := engine.Validate(policyContext)
		if engineResponse.IsNil() {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}

		go webhookutils.RegisterPolicyResultsMetricValidation(logger, metricsConfig, string(request.Operation), policyContext.Policy, *engineResponse)
		go webhookutils.RegisterPolicyExecutionDurationMetricValidate(logger, metricsConfig, string(request.Operation), policyContext.Policy, *engineResponse)

		engineResponses = append(engineResponses, engineResponse)
		if !engineResponse.IsSuccessful() {
			logger.V(2).Info("validation failed", "action", policy.GetSpec().ValidationFailureAction, "policy", policy.GetName(), "failed rules", engineResponse.GetFailedRules())
			continue
		}

		if len(engineResponse.GetSuccessRules()) > 0 {
			logger.V(2).Info("validation passed", "policy", policy.GetName())
		}
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

	go v.handleAudit(policyContext.NewResource, request, namespaceLabels, engineResponses...)

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings
}

func (v *validationHandler) buildAuditResponses(resource unstructured.Unstructured, request *admissionv1.AdmissionRequest, namespaceLabels map[string]string) ([]*response.EngineResponse, error) {
	policies := v.pCache.GetPolicies(policycache.ValidateAudit, request.Kind.Kind, request.Namespace)
	policyContext, err := v.pcBuilder.Build(request, policies...)
	if err != nil {
		return nil, err
	}
	var responses []*response.EngineResponse
	for _, policy := range policies {
		policyContext.Policy = policy
		policyContext.NamespaceLabels = namespaceLabels
		responses = append(responses, engine.Validate(policyContext))
	}
	return responses, nil
}

func (v *validationHandler) handleAudit(
	resource unstructured.Unstructured,
	request *admissionv1.AdmissionRequest,
	namespaceLabels map[string]string,
	engineResponses ...*response.EngineResponse,
) {
	if !v.admissionReports {
		return
	}
	if request.DryRun != nil && *request.DryRun {
		return
	}
	// we don't need reports for deletions and when it's about sub resources
	if request.Operation == admissionv1.Delete || request.SubResource != "" {
		return
	}
	// check if the resource supports reporting
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Kind)) {
		return
	}
	responses, err := v.buildAuditResponses(resource, request, namespaceLabels)
	if err != nil {
		v.log.Error(err, "failed to build audit responses")
	}
	responses = append(responses, engineResponses...)
	report := reportutils.BuildAdmissionReport(resource, request, request.Kind, responses...)
	// if it's not a creation, the resource already exists, we can set the owner
	if request.Operation != admissionv1.Create {
		gv := metav1.GroupVersion{Group: request.Kind.Group, Version: request.Kind.Version}
		controllerutils.SetOwner(report, gv.String(), request.Kind.Kind, resource.GetName(), resource.GetUID())
	}
	if len(report.GetResults()) > 0 {
		_, err = reportutils.CreateReport(context.Background(), report, v.kyvernoClient)
		if err != nil {
			v.log.Error(err, "failed to create report")
		}
	}
}
