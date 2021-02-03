package webhooks

import (
	"reflect"
	"sort"
	"time"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HandleMutation handles mutating webhook admission request
// return value: generated patches
func (ws *WebhookServer) HandleMutation(
	request *v1beta1.AdmissionRequest,
	resource unstructured.Unstructured,
	policies []*kyverno.ClusterPolicy,
	ctx *context.Context,
	userRequestInfo kyverno.RequestInfo) []byte {

	if len(policies) == 0 {
		return nil
	}

	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	logger := ws.log.WithValues("action", "mutate", "resource", resourceName, "operation", request.Operation)

	var patches [][]byte
	var engineResponses []*response.EngineResponse
	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    ws.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configHandler.ToFilter,
		ResourceCache:       ws.resCache,
		JSONContext:         ctx,
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
			logger.Info("failed to apply policy", "policy", policy.Name, "failed rules", engineResponse.GetFailedRules())
			continue
		}

		err := ws.openAPIController.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetKind())
		if err != nil {
			logger.V(4).Info("validation error", "policy", policy.Name, "error", err.Error())
			continue
		}

		if len(policyPatches) > 0 {
			patches = append(patches, policyPatches...)
			rules := engineResponse.GetSuccessRules()
			logger.Info("mutation rules from policy applied successfully", "policy", policy.Name, "rules", rules)
		}

		policyContext.NewResource = engineResponse.PatchedResource
		engineResponses = append(engineResponses, engineResponse)
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
			logger.Info("failed to apply mutation rules on the resource, reporting policy violation", "errors", getErrorMsg(engineResponses))
		}
	}()

	// patches holds all the successful patches, if no patch is created, it returns nil
	return engineutils.JoinPatches(patches)
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
