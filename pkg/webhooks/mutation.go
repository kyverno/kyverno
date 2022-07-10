package webhooks

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/utils"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (ws *WebhookServer) applyMutatePolicies(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []kyverno.PolicyInterface, ts int64, logger logr.Logger) []byte {
	var mutateEngineResponses []*response.EngineResponse

	mutatePatches, mutateEngineResponses := ws.handleMutation(request, policyContext, policies)
	logger.V(6).Info("", "generated patches", string(mutatePatches))

	admissionReviewLatencyDuration := int64(time.Since(time.Unix(ts, 0)))
	go ws.registerAdmissionReviewDurationMetricMutate(logger, string(request.Operation), mutateEngineResponses, admissionReviewLatencyDuration)
	go ws.registerAdmissionRequestsMetricMutate(logger, string(request.Operation), mutateEngineResponses)

	return mutatePatches
}

// handleMutation handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (ws *WebhookServer) handleMutation(
	request *admissionv1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []kyverno.PolicyInterface) ([]byte, []*response.EngineResponse) {

	if len(policies) == 0 {
		return nil, nil
	}

	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	logger := ws.log.WithValues("action", "mutate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	patchedResource := request.Object.Raw
	newR, oldR, err := utils.ExtractResources(patchedResource, request)
	if err != nil {
		// as resource cannot be parsed, we skip processing
		logger.Error(err, "failed to extract resource")
		return nil, nil
	}
	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
		deletionTimeStamp = newR.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = oldR.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == admissionv1.Update {
		return nil, nil
	}
	var patches [][]byte
	var engineResponses []*response.EngineResponse

	for _, policy := range policies {
		spec := policy.GetSpec()
		if !spec.HasMutate() {
			continue
		}
		logger.V(3).Info("applying policy mutate rules", "policy", policy.GetName())
		policyContext.Policy = policy
		engineResponse, policyPatches, err := ws.applyMutation(request, policyContext, logger)
		if err != nil {
			// TODO report errors in engineResponse and record in metrics
			logger.Error(err, "mutate error")
			continue
		}

		if len(policyPatches) > 0 {
			patches = append(patches, policyPatches...)
			rules := engineResponse.GetSuccessRules()
			if len(rules) != 0 {
				logger.Info("mutation rules from policy applied successfully", "policy", policy.GetName(), "rules", rules)
			}
		}

		policyContext.NewResource = engineResponse.PatchedResource
		engineResponses = append(engineResponses, engineResponse)

		// registering the kyverno_policy_results_total metric concurrently
		go ws.registerPolicyResultsMetricMutation(logger, string(request.Operation), policy, *engineResponse)

		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go ws.registerPolicyExecutionDurationMetricMutate(logger, string(request.Operation), policy, *engineResponse)
	}

	// generate annotations
	if annPatches := utils.GenerateAnnotationPatches(engineResponses, logger); annPatches != nil {
		patches = append(patches, annPatches...)
	}

	// REPORTING EVENTS
	// Scenario 1:
	//   some/all policies failed to apply on the resource. a policy violation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 2:
	//   all policies were applied successfully.
	//   create an event on the resource
	// ADD EVENTS
	if deletionTimeStamp == nil {
		events := generateEvents(engineResponses, false, logger)
		ws.eventGen.Add(events...)
	}

	// debug info
	func() {
		if len(patches) != 0 {
			logger.V(4).Info("JSON patches generated")
		}

		// if any of the policies fails, print out the error
		if !engineutils.IsResponseSuccessful(engineResponses) {
			logger.Error(errors.New(getErrorMsg(engineResponses)), "failed to apply mutation rules on the resource, reporting policy violation")
		}
	}()

	// patches holds all the successful patches, if no patch is created, it returns nil
	return jsonutils.JoinPatches(patches...), engineResponses
}

func (ws *WebhookServer) applyMutation(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, logger logr.Logger) (*response.EngineResponse, [][]byte, error) {
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(
			request.Kind.Kind, request.Namespace, ws.nsLister, logger)
	}

	engineResponse := engine.Mutate(policyContext)
	policyPatches := engineResponse.GetPatches()

	if !engineResponse.IsSuccessful() && len(engineResponse.GetFailedRules()) > 0 {
		return nil, nil, fmt.Errorf("failed to apply policy %s rules %v", policyContext.Policy.GetName(), engineResponse.GetFailedRules())
	}

	if engineResponse.PatchedResource.GetKind() != "*" {
		err := ws.openAPIController.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetAPIVersion(), engineResponse.PatchedResource.GetKind())
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to validate resource mutated by policy %s", policyContext.Policy.GetName())
		}
	}

	return engineResponse, policyPatches, nil
}
