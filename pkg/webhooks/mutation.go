package webhooks

import (
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyRuleExecutionLatency "github.com/kyverno/kyverno/pkg/metrics/policyruleexecutionlatency"
	policyRuleResults "github.com/kyverno/kyverno/pkg/metrics/policyruleresults"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HandleMutation handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (ws *WebhookServer) HandleMutation(
	request *v1beta1.AdmissionRequest,
	resource unstructured.Unstructured,
	policies []*kyverno.ClusterPolicy,
	ctx *context.Context,
	userRequestInfo kyverno.RequestInfo,
	admissionRequestTimestamp int64) ([]byte, []kyverno.ClusterPolicy, []*response.EngineResponse) {

	if len(policies) == 0 {
		return nil, nil, nil
	}

	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	logger := ws.log.WithValues("action", "mutate", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var patches [][]byte
	var engineResponses []*response.EngineResponse
	var triggeredPolicies []kyverno.ClusterPolicy
	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    ws.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configHandler.ToFilter,
		ResourceCache:       ws.resCache,
		JSONContext:         ctx,
		Client:              ws.client,
	}

	if request.Operation == v1beta1.Update {
		// set OldResource to inform engine of operation type
		policyContext.OldResource = resource
	}

	for _, policy := range policies {
		logger.V(3).Info("evaluating policy", "policy", policy.Name)

		policyContext.Policy = *policy
		if request.Kind.Kind != "Namespace" && request.Namespace != "" {
			policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, ws.nsLister, logger)
		}
		engineResponse := engine.Mutate(policyContext)
		policyPatches := engineResponse.GetPatches()

		if engineResponse.PolicyResponse.RulesAppliedCount > 0 && len(policyPatches) > 0 {
			ws.statusListener.Update(mutateStats{resp: engineResponse, namespace: policy.Namespace})
		}

		if !engineResponse.IsSuccessful() && len(engineResponse.GetFailedRules()) > 0 {
			logger.Error(errors.New("some rules failed"), "failed to apply policy", "policy", policy.Name, "failed rules", engineResponse.GetFailedRules())
			continue
		}

		err := ws.openAPIController.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetAPIVersion(), engineResponse.PatchedResource.GetKind())
		if err != nil {
			logger.Info("validation error", "policy", policy.Name, "error", err.Error())
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

type mutateStats struct {
	resp      *response.EngineResponse
	namespace string
}

func (ms mutateStats) PolicyName() string {
	if ms.namespace == "" {
		return ms.resp.PolicyResponse.Policy
	}
	return ms.namespace + "/" + ms.resp.PolicyResponse.Policy
}

func (ms mutateStats) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {
	if reflect.DeepEqual(response.EngineResponse{}, ms.resp) {
		return status
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range status.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range ms.resp.PolicyResponse.Rules {
		ruleStat := nameToRule[rule.Name]
		ruleStat.Name = rule.Name

		averageOver := int64(ruleStat.AppliedCount + ruleStat.FailedCount)
		ruleStat.ExecutionTime = updateAverageTime(
			rule.ProcessingTime,
			ruleStat.ExecutionTime,
			averageOver).String()

		if rule.Success {
			status.RulesAppliedCount++
			status.ResourcesMutatedCount++
			ruleStat.AppliedCount++
			ruleStat.ResourcesMutatedCount++
		} else {
			status.RulesFailedCount++
			ruleStat.FailedCount++
		}

		nameToRule[rule.Name] = ruleStat
	}

	var policyAverageExecutionTime time.Duration
	var ruleStats = make([]v1.RuleStats, 0, len(nameToRule))
	for _, ruleStat := range nameToRule {
		executionTime, err := time.ParseDuration(ruleStat.ExecutionTime)
		if err == nil {
			policyAverageExecutionTime += executionTime
		}
		ruleStats = append(ruleStats, ruleStat)
	}

	sort.Slice(ruleStats, func(i, j int) bool {
		return ruleStats[i].Name < ruleStats[j].Name
	})

	status.AvgExecutionTime = policyAverageExecutionTime.String()
	status.Rules = ruleStats

	return status
}
