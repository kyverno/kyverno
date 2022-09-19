package validation

import (
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
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

func NewValidationHandler(log logr.Logger, eventGen event.Interface) ValidationHandler {
	return &validationHandler{
		log:      log,
		eventGen: eventGen,
	}
}

type validationHandler struct {
	log      logr.Logger
	eventGen event.Interface
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
		logger.V(3).Info("evaluating policy", "policy", policy.GetName())
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

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", warnings
}

func (v *validationHandler) generateMetrics(request *admissionv1.AdmissionRequest, admissionRequestTimestamp time.Time, engineResponses []*response.EngineResponse, metricsConfig *metrics.MetricsConfig, logger logr.Logger) {
	admissionReviewLatencyDuration := int64(time.Since(admissionRequestTimestamp))
	go webhookutils.RegisterAdmissionReviewDurationMetricValidate(logger, metricsConfig, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
	go webhookutils.RegisterAdmissionRequestsMetricValidate(logger, metricsConfig, string(request.Operation), engineResponses)
}
