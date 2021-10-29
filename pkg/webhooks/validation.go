package webhooks

import (
	"reflect"
	"time"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/event"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionRequests "github.com/kyverno/kyverno/pkg/metrics/admissionrequests"
	admissionReviewDuration "github.com/kyverno/kyverno/pkg/metrics/admissionreviewduration"
	policyExecutionDuration "github.com/kyverno/kyverno/pkg/metrics/policyexecutionduration"
	policyResults "github.com/kyverno/kyverno/pkg/metrics/policyresults"
	"github.com/kyverno/kyverno/pkg/policyreport"
	v1beta1 "k8s.io/api/admission/v1beta1"
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
	promConfig *metrics.PromConfig,
	request *v1beta1.AdmissionRequest,
	policies []*v1.ClusterPolicy,
	policyContext *engine.PolicyContext,
	namespaceLabels map[string]string,
	admissionRequestTimestamp int64) (bool, string) {

	if len(policies) == 0 {
		return true, ""
	}

	resourceName := getResourceName(request)
	logger := v.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource, unstructured.Unstructured{}) {
		deletionTimeStamp = policyContext.NewResource.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = policyContext.OldResource.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == v1beta1.Update {
		return true, ""
	}

	var engineResponses []*response.EngineResponse
	for _, policy := range policies {
		logger.V(3).Info("evaluating policy", "policy", policy.Name)
		policyContext.Policy = *policy
		policyContext.NamespaceLabels = namespaceLabels
		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}

		// registering the kyverno_policy_results_total metric concurrently
		go registerPolicyResultsMetricValidation(promConfig, logger, string(request.Operation), policyContext.Policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go registerPolicyExecutionDurationMetricValidate(promConfig, logger, string(request.Operation), policyContext.Policy, *engineResponse)

		engineResponses = append(engineResponses, engineResponse)
		if !engineResponse.IsSuccessful() {
			logger.V(2).Info("validation failed", "policy", policy.Name, "failed rules", engineResponse.GetFailedRules())
			continue
		}

		if len(engineResponse.GetSuccessRules()) > 0 {
			logger.V(2).Info("validation passed", "policy", policy.Name)
		}
	}

	// If Validation fails then reject the request
	// no violations will be created on "enforce"
	blocked := toBlockResource(engineResponses, logger)

	// REPORTING EVENTS
	// Scenario 1:
	//   resource is blocked, as there is a policy in "enforce" mode that failed.
	//   create an event on the policy to inform the resource request was blocked
	// Scenario 2:
	//   some/all policies failed to apply on the resource. a policy violation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 3:
	//   all policies were applied successfully.
	//   create an event on the resource
	events := generateEvents(engineResponses, blocked, (request.Operation == v1beta1.Update), logger)
	v.eventGen.Add(events...)
	if blocked {
		logger.V(4).Info("resource blocked")
		//registering the kyverno_admission_review_duration_seconds metric concurrently
		admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
		go registerAdmissionReviewDurationMetricValidate(promConfig, logger, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
		//registering the kyverno_admission_requests_total metric concurrently
		go registerAdmissionRequestsMetricValidate(promConfig, logger, string(request.Operation), engineResponses)
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	if request.Operation == v1beta1.Delete {
		v.prGenerator.Add(buildDeletionPrInfo(policyContext.OldResource))
		return true, ""
	}

	prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
	v.prGenerator.Add(prInfos...)

	//registering the kyverno_admission_review_duration_seconds metric concurrently
	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	go registerAdmissionReviewDurationMetricValidate(promConfig, logger, string(request.Operation), engineResponses, admissionReviewLatencyDuration)

	//registering the kyverno_admission_requests_total metric concurrently
	go registerAdmissionRequestsMetricValidate(promConfig, logger, string(request.Operation), engineResponses)
	return true, ""
}

func getResourceName(request *v1beta1.AdmissionRequest) string {
	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	return resourceName
}

func registerPolicyResultsMetricValidation(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, policy v1.ClusterPolicy, engineResponse response.EngineResponse) {
	resourceRequestOperationPromAlias, err := policyResults.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.Name)
	}
	if err := policyResults.ParsePromConfig(*promConfig).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.Name)
	}
}

func registerPolicyExecutionDurationMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, policy v1.ClusterPolicy, engineResponse response.EngineResponse) {
	resourceRequestOperationPromAlias, err := policyExecutionDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.Name)
	}
	if err := policyExecutionDuration.ParsePromConfig(*promConfig).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, "", resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.Name)
	}
}

func registerAdmissionReviewDurationMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse, admissionReviewLatencyDuration int64) {
	resourceRequestOperationPromAlias, err := admissionReviewDuration.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
	if err := admissionReviewDuration.ParsePromConfig(*promConfig).ProcessEngineResponses(engineResponses, admissionReviewLatencyDuration, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_review_duration_seconds metrics")
	}
}

func registerAdmissionRequestsMetricValidate(promConfig *metrics.PromConfig, logger logr.Logger, requestOperation string, engineResponses []*response.EngineResponse) {
	resourceRequestOperationPromAlias, err := admissionRequests.ParseResourceRequestOperation(requestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
	if err := admissionRequests.ParsePromConfig(*promConfig).ProcessEngineResponses(engineResponses, resourceRequestOperationPromAlias); err != nil {
		logger.Error(err, "error occurred while registering kyverno_admission_requests_total metrics")
	}
}

func buildDeletionPrInfo(oldR unstructured.Unstructured) policyreport.Info {
	return policyreport.Info{
		Namespace: oldR.GetNamespace(),
		Results: []policyreport.EngineResponseResult{
			{Resource: response.ResourceSpec{
				Kind:       oldR.GetKind(),
				APIVersion: oldR.GetAPIVersion(),
				Namespace:  oldR.GetNamespace(),
				Name:       oldR.GetName(),
				UID:        string(oldR.GetUID()),
			}},
		},
	}
}
