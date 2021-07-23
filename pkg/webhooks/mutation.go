package webhooks

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kyverno/kyverno/pkg/common"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyRuleExecutionLatency "github.com/kyverno/kyverno/pkg/metrics/policyruleexecutionlatency"
	policyRuleResults "github.com/kyverno/kyverno/pkg/metrics/policyruleresults"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (ws *WebhookServer) applyMutatePolicies(request *v1beta1.AdmissionRequest, policyContext *engine.PolicyContext, policies []*v1.ClusterPolicy, ts int64, logger logr.Logger) []byte {
	var triggeredMutatePolicies []v1.ClusterPolicy
	var mutateEngineResponses []*response.EngineResponse

	mutatePatches, triggeredMutatePolicies, mutateEngineResponses := ws.handleMutation(request, policyContext, policies, ts)
	logger.V(6).Info("", "generated patches", string(mutatePatches))

	admissionReviewLatencyDuration := int64(time.Since(time.Unix(ts, 0)))
	go registerAdmissionReviewLatencyMetricMutate(logger, *ws.promConfig.Metrics, string(request.Operation), mutateEngineResponses, triggeredMutatePolicies, admissionReviewLatencyDuration, ts)

	return mutatePatches
}

// handleMutation handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (ws *WebhookServer) handleMutation(
	request *v1beta1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []*kyverno.ClusterPolicy,
	admissionRequestTimestamp int64) ([]byte, []kyverno.ClusterPolicy, []*response.EngineResponse) {

	if len(policies) == 0 {
		return nil, nil, nil
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
		return nil, nil, nil
	}
	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
		deletionTimeStamp = newR.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = oldR.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == v1beta1.Update {
		return nil, nil, nil
	}
	var patches [][]byte
	var engineResponses []*response.EngineResponse
	var triggeredPolicies []kyverno.ClusterPolicy

	for _, policy := range policies {
		if !policy.HasMutate() {
			continue
		}

		logger.V(3).Info("applying policy mutate rules", "policy", policy.Name)
		policyContext.Policy = *policy
		engineResponse, policyPatches, err := ws.applyMutation(request, policyContext, logger)
		if err != nil {
			// TODO report errors in engineResponse and record in metrics
			logger.Error(err, "mutate error")
			continue
		}

		if len(policyPatches) > 0 {
			patches = append(patches, policyPatches...)
			rules := engineResponse.GetSuccessRules()
			logger.Info("mutation rules from policy applied successfully", "policy", policy.Name, "rules", rules)
		}

		policyContext.NewResource = engineResponse.PatchedResource
		engineResponses = append(engineResponses, engineResponse)

		// registering the kyverno_policy_rule_results_info metric concurrently
		go ws.registerPolicyRuleResultsMetricMutation(logger, string(request.Operation), *policy, *engineResponse, admissionRequestTimestamp)
		triggeredPolicies = append(triggeredPolicies, *policy)

		// registering the kyverno_policy_rule_execution_latency_milliseconds metric concurrently
		go ws.registerPolicyRuleExecutionLatencyMetricMutate(logger, string(request.Operation), *policy, *engineResponse, admissionRequestTimestamp)
		triggeredPolicies = append(triggeredPolicies, *policy)
	}

	// generate annotations
	if annPatches := generateAnnotationPatches(engineResponses, logger); annPatches != nil {
		patches = append(patches, annPatches)
	}

	// REPORTING EVENTS
	// Scenario 1:
	//   some/all policies failed to apply on the resource. a policy violation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 2:
	//   all policies were applied successfully.
	//   create an event on the resource
	// ADD EVENTS
	events := generateEvents(engineResponses, false, (request.Operation == v1beta1.Update), logger)
	ws.eventGen.Add(events...)

	// debug info
	func() {
		if len(patches) != 0 {
			logger.V(4).Info("JSON patches generated")
		}

		// if any of the policies fails, print out the error
		if !isResponseSuccessful(engineResponses) {
			logger.Error(errors.New(getErrorMsg(engineResponses)), "failed to apply mutation rules on the resource, reporting policy violation")
		}
	}()

	// patches holds all the successful patches, if no patch is created, it returns nil
	return engineutils.JoinPatches(patches), triggeredPolicies, engineResponses
}

func (ws *WebhookServer) applyMutation(request *v1beta1.AdmissionRequest, policyContext *engine.PolicyContext, logger logr.Logger) (*response.EngineResponse, [][]byte, error) {
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(
			request.Kind.Kind, request.Namespace, ws.nsLister, logger)
	}

	engineResponse := engine.Mutate(policyContext)
	policyPatches := engineResponse.GetPatches()

	if !engineResponse.IsSuccessful() && len(engineResponse.GetFailedRules()) > 0 {
		return nil, nil, fmt.Errorf("failed to apply policy %s rules %v", policyContext.Policy.Name, engineResponse.GetFailedRules())
	}

	err := ws.openAPIController.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetAPIVersion(), engineResponse.PatchedResource.GetKind())
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to validate resource mutated by policy %s", policyContext.Policy.Name)
	}

	return engineResponse, policyPatches, nil
}

func (ws *WebhookServer) registerPolicyRuleResultsMetricMutation(logger logr.Logger, resourceRequestOperation string, policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := policyRuleResults.ParseResourceRequestOperation(resourceRequestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_results_info metrics for the above policy", "name", policy.Name)
	}
	if err := policyRuleResults.ParsePromMetrics(*ws.promConfig.Metrics).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_results_info metrics for the above policy", "name", policy.Name)
	}
}

func (ws *WebhookServer) registerPolicyRuleExecutionLatencyMetricMutate(logger logr.Logger, resourceRequestOperation string, policy kyverno.ClusterPolicy, engineResponse response.EngineResponse, admissionRequestTimestamp int64) {
	resourceRequestOperationPromAlias, err := policyRuleExecutionLatency.ParseResourceRequestOperation(resourceRequestOperation)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics for the above policy", "name", policy.Name)
	}
	if err := policyRuleExecutionLatency.ParsePromMetrics(*ws.promConfig.Metrics).ProcessEngineResponse(policy, engineResponse, metrics.AdmissionRequest, "", resourceRequestOperationPromAlias, admissionRequestTimestamp); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_execution_latency_milliseconds metrics for the above policy", "name", policy.Name)
	}
}
