package webhooks

import (
	"reflect"
	"sort"
	"time"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/policyviolation"
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

	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	logger := ws.log.WithValues("action", "mutate", "resource", resourceName, "operation", request.Operation)

	var patches [][]byte
	var engineResponses []response.EngineResponse
	policyContext := engine.PolicyContext{
		NewResource:   resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
	}

	for _, policy := range policies {
		logger.V(3).Info("evaluating policy", "policy", policy.Name)

		policyContext.Policy = *policy
		engineResponse := engine.Mutate(policyContext)

		engineResponses = append(engineResponses, engineResponse)
		ws.statusListener.Send(mutateStats{resp: engineResponse})
		if !engineResponse.IsSuccesful() {
			logger.Info("failed to apply policy", "policy", policy.Name, "failed rules", engineResponse.GetFailedRules())
			continue
		}

		err := ws.openAPIController.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetKind())
		if err != nil {
			logger.Error(err, "validation error", "policy", policy.Name)
			continue
		}

		// gather patches
		patches = append(patches, engineResponse.GetPatches()...)
		if len(engineResponse.GetPatches()) != 0 {
			logger.Info("mutation rules from policy applied succesfully", "policy", policy.Name)
		}

		policyContext.NewResource = engineResponse.PatchedResource
	}

	// generate annotations
	if annPatches := generateAnnotationPatches(engineResponses, logger); annPatches != nil {
		patches = append(patches, annPatches)
	}

	// AUDIT
	// generate violation when response fails
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses, logger)
	ws.pvGenerator.Add(pvInfos...)

	// REPORTING EVENTS
	// Scenario 1:
	//   some/all policies failed to apply on the resource. a policy volation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 2:
	//   all policies were applied succesfully.
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
		if !isResponseSuccesful(engineResponses) {
			logger.Info("failed to apply mutation rules on the resource, reporting policy violation", "errors", getErrorMsg(engineResponses))
		}
	}()

	// patches holds all the successful patches, if no patch is created, it returns nil
	return engineutils.JoinPatches(patches)
}

type mutateStats struct {
	resp response.EngineResponse
}

func (ms mutateStats) PolicyName() string {
	return ms.resp.PolicyResponse.Policy
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
