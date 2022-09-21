package validation

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	auditcontroller "github.com/kyverno/kyverno/pkg/controllers/audit"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policycache"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ValidationHandler interface {
	// HandleValidation handles validating webhook admission request
	// If there are no errors in validating rule we apply generation rules
	// patchedResource is the (resource + patches) after applying mutation rules
	HandleValidation(*metrics.MetricsConfig, *admissionv1.AdmissionRequest, []kyvernov1.PolicyInterface, *engine.PolicyContext, map[string]string, time.Time) (bool, string, []string)
}

func NewValidationHandler(log logr.Logger, kyvernoClient versioned.Interface, pCache policycache.Cache, pcBuilder webhookutils.PolicyContextBuilder, eventGen event.Interface) ValidationHandler {
	return &validationHandler{
		log:           log,
		kyvernoClient: kyvernoClient,
		pCache:        pCache,
		pcBuilder:     pcBuilder,
		eventGen:      eventGen,
	}
}

type validationHandler struct {
	log           logr.Logger
	kyvernoClient versioned.Interface
	pCache        policycache.Cache
	pcBuilder     webhookutils.PolicyContextBuilder
	eventGen      event.Interface
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
		if request.Operation != admissionv1.Delete && request.SubResource == "" {
			go v.handleAudit(policyContext.NewResource, request, namespaceLabels)
		}
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
			logger.V(2).Info("validation failed", "policy", policy.GetName(), "failed rules", engineResponse.GetFailedRules())
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
		v.generateMetrics(request, admissionRequestTimestamp, engineResponses, metricsConfig, logger)
		return false, webhookutils.GetBlockedMessages(engineResponses), nil
	}

	v.generateMetrics(request, admissionRequestTimestamp, engineResponses, metricsConfig, logger)
	if request.Operation != admissionv1.Delete && request.SubResource == "" {
		go v.handleAudit(policyContext.NewResource, request, namespaceLabels, engineResponses...)
	}

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings
}

func (v *validationHandler) generateMetrics(request *admissionv1.AdmissionRequest, admissionRequestTimestamp time.Time, engineResponses []*response.EngineResponse, metricsConfig *metrics.MetricsConfig, logger logr.Logger) {
	admissionReviewLatencyDuration := int64(time.Since(admissionRequestTimestamp))
	go webhookutils.RegisterAdmissionReviewDurationMetricValidate(logger, metricsConfig, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
	go webhookutils.RegisterAdmissionRequestsMetricValidate(logger, metricsConfig, string(request.Operation), engineResponses)
}

func (v *validationHandler) buildReport(
	report kyvernov1alpha2.ReportChangeRequestInterface,
	resource unstructured.Unstructured,
	request *admissionv1.AdmissionRequest,
	namespaceLabels map[string]string,
	engineResponses ...*response.EngineResponse,
) error {
	v.log.Info("in buildReport...", "gvk", request.Kind, "ns", request.Namespace)
	policies := v.pCache.GetPolicies(policycache.ValidateAudit, request.Kind.Kind, request.Namespace)
	policyContext, err := v.pcBuilder.Build(request, policies...)
	if err != nil {
		return err
	}
	var responses []*response.EngineResponse
	responses = append(responses, engineResponses...)
	for _, policy := range policies {
		v.log.Info("admission report...", "policy", policy.GetName())
		policyContext.Policy = policy
		policyContext.NamespaceLabels = namespaceLabels
		responses = append(responses, engine.Validate(policyContext))
	}
	err = auditcontroller.BuildReport(report, request.Kind.Group, request.Kind.Version, request.Kind.Kind, &resource, responses...)
	if err != nil {
		return err
	}
	controllerutils.SetLabel(report, "audit.kyverno.io/request.group", request.Kind.Group)
	controllerutils.SetLabel(report, "audit.kyverno.io/request.version", request.Kind.Version)
	controllerutils.SetLabel(report, "audit.kyverno.io/request.kind", request.Kind.Kind)
	controllerutils.SetLabel(report, "audit.kyverno.io/request.namespace", request.Namespace)
	controllerutils.SetLabel(report, "audit.kyverno.io/request.name", request.Name)
	controllerutils.SetLabel(report, "audit.kyverno.io/request.uid", string(request.UID))
	if request.Operation != admissionv1.Create {
		gv := metav1.GroupVersion{Group: request.Kind.Group, Version: request.Kind.Version}
		controllerutils.SetOwner(report, gv.String(), request.Kind.Kind, resource.GetName(), resource.GetUID())
	}
	return nil
}

func (v *validationHandler) handleAudit(
	resource unstructured.Unstructured,
	request *admissionv1.AdmissionRequest,
	namespaceLabels map[string]string,
	engineResponses ...*response.EngineResponse,
) {
	namespace := resource.GetNamespace()
	if namespace == "" {
		report := &kyvernov1alpha2.ClusterReportChangeRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(request.UID),
			},
		}
		err := v.buildReport(report, resource, request, namespaceLabels, engineResponses...)
		if err == nil {
			_, err = v.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests().Create(context.TODO(), report, metav1.CreateOptions{})
			if err != nil {
				v.log.Error(err, "failed to create report")
			}
		} else {
			v.log.Error(err, "failed to build report")
		}
	} else {
		report := &kyvernov1alpha2.ReportChangeRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(request.UID),
				Namespace: request.Namespace,
			},
		}
		err := v.buildReport(report, resource, request, namespaceLabels, engineResponses...)
		if err == nil {
			_, err = v.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(report.Namespace).Create(context.TODO(), report, metav1.CreateOptions{})
			if err != nil {
				v.log.Error(err, "failed to create report")
			}
		} else {
			v.log.Error(err, "failed to build report")
		}
	}
}
