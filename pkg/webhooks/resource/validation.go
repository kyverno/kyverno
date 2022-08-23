package resource

import (
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policyreport"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type validationHandler struct {
	log         logr.Logger
	eventGen    event.Interface
	prGenerator policyreport.GeneratorInterface
}

// handleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (v *validationHandler) handleValidation(
	metricsConfig *metrics.MetricsConfig,
	request *admissionv1.AdmissionRequest,
	policies []kyvernov1.PolicyInterface,
	policyContext *engine.PolicyContext,
	namespaceLabels map[string]string,
	admissionRequestTimestamp int64,
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

		go registerPolicyResultsMetricValidation(logger, metricsConfig, string(request.Operation), policyContext.Policy, *engineResponse)
		go registerPolicyExecutionDurationMetricValidate(logger, metricsConfig, string(request.Operation), policyContext.Policy, *engineResponse)

		engineResponses = append(engineResponses, engineResponse)
		if !engineResponse.IsSuccessful() {
			logger.V(2).Info("validation failed", "policy", policy.GetName(), "failed rules", engineResponse.GetFailedRules())
			continue
		}

		if len(engineResponse.GetSuccessRules()) > 0 {
			logger.V(2).Info("validation passed", "policy", policy.GetName())
		}
	}

	blocked := blockRequest(engineResponses, failurePolicy, logger)
	if deletionTimeStamp == nil {
		events := generateEvents(engineResponses, blocked, logger)
		v.eventGen.Add(events...)
	}

	if blocked {
		logger.V(4).Info("admission request blocked")
		v.generateMetrics(request, admissionRequestTimestamp, engineResponses, metricsConfig, logger)
		return false, getBlockedMessages(engineResponses), nil
	}

	v.generateReportChangeRequests(request, engineResponses, policyContext, logger)
	v.generateMetrics(request, admissionRequestTimestamp, engineResponses, metricsConfig, logger)

	warnings := getWarningMessages(engineResponses)
	return true, "", warnings
}

// generateReportChangeRequests creates report change requests
// reports are generated for non-managed pods/jobs only, no need to create rcr for managed resources
func (v *validationHandler) generateReportChangeRequests(request *admissionv1.AdmissionRequest, engineResponses []*response.EngineResponse, policyContext *engine.PolicyContext, logger logr.Logger) {
	if request.Operation == admissionv1.Delete {
		managed := true
		for _, er := range engineResponses {
			if er.Policy != nil && !engine.ManagedPodResource(er.Policy, er.PatchedResource) {
				managed = false
				break
			}
		}

		if !managed {
			v.prGenerator.Add(buildDeletionPrInfo(policyContext.OldResource))
		}
	} else {
		prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
		v.prGenerator.Add(prInfos...)
	}
}

func (v *validationHandler) generateMetrics(request *admissionv1.AdmissionRequest, admissionRequestTimestamp int64, engineResponses []*response.EngineResponse, metricsConfig *metrics.MetricsConfig, logger logr.Logger) {
	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	go registerAdmissionReviewDurationMetricValidate(logger, metricsConfig, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
	go registerAdmissionRequestsMetricValidate(logger, metricsConfig, string(request.Operation), engineResponses)
}
