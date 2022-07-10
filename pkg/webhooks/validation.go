package webhooks

import (
	"reflect"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	promConfig *metrics.PromConfig,
	request *admissionv1.AdmissionRequest,
	policies []v1.PolicyInterface,
	policyContext *engine.PolicyContext,
	namespaceLabels map[string]string,
	admissionRequestTimestamp int64) (bool, string) {

	if len(policies) == 0 {
		return true, ""
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
		return true, ""
	}

	var engineResponses []*response.EngineResponse
	for _, policy := range policies {
		logger.V(3).Info("evaluating policy", "policy", policy.GetName())
		policyContext.Policy = policy
		policyContext.NamespaceLabels = namespaceLabels
		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}

		// registering the kyverno_policy_results_total metric concurrently
		go registerPolicyResultsMetricValidation(logger, promConfig, string(request.Operation), policyContext.Policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go registerPolicyExecutionDurationMetricValidate(logger, promConfig, string(request.Operation), policyContext.Policy, *engineResponse)

		engineResponses = append(engineResponses, engineResponse)
		if !engineResponse.IsSuccessful() {
			logger.V(2).Info("validation failed", "policy", policy.GetName(), "failed rules", engineResponse.GetFailedRules())
			continue
		}

		if len(engineResponse.GetSuccessRules()) > 0 {
			logger.V(2).Info("validation passed", "policy", policy.GetName())
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
	if deletionTimeStamp == nil {
		events := generateEvents(engineResponses, blocked, logger)
		v.eventGen.Add(events...)
	}

	if blocked {
		logger.V(4).Info("resource blocked")
		//registering the kyverno_admission_review_duration_seconds metric concurrently
		admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
		go registerAdmissionReviewDurationMetricValidate(logger, promConfig, string(request.Operation), engineResponses, admissionReviewLatencyDuration)
		//registering the kyverno_admission_requests_total metric concurrently
		go registerAdmissionRequestsMetricValidate(logger, promConfig, string(request.Operation), engineResponses)
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	// reports are generated for non-managed pods/jobs only
	// no need to create rcr for managed resources
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

		return true, ""
	}

	prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
	v.prGenerator.Add(prInfos...)

	//registering the kyverno_admission_review_duration_seconds metric concurrently
	admissionReviewLatencyDuration := int64(time.Since(time.Unix(admissionRequestTimestamp, 0)))
	go registerAdmissionReviewDurationMetricValidate(logger, promConfig, string(request.Operation), engineResponses, admissionReviewLatencyDuration)

	//registering the kyverno_admission_requests_total metric concurrently
	go registerAdmissionRequestsMetricValidate(logger, promConfig, string(request.Operation), engineResponses)
	return true, ""
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
