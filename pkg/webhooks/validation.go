package webhooks

import (
	"reflect"
	"sort"
	"time"

	"github.com/nirmata/kyverno/pkg/utils"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (ws *WebhookServer) HandleValidation(
	request *v1beta1.AdmissionRequest,
	policies []kyverno.ClusterPolicy,
	patchedResource []byte,
	ctx *context.Context,
	userRequestInfo kyverno.RequestInfo) (bool, string) {

	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}

	logger := ws.log.WithValues("action", "validate", "resource", resourceName, "operation", request.Operation)

	// Get new and old resource
	newR, oldR, err := utils.ExtractResources(patchedResource, request)
	if err != nil {
		// as resource cannot be parsed, we skip processing
		logger.Error(err, "failed to extract resource")
		return true, ""
	}

	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(newR, unstructured.Unstructured{}) {
		deletionTimeStamp = newR.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = oldR.GetDeletionTimestamp()
	}

	if deletionTimeStamp != nil && request.Operation == v1beta1.Update {
		return true, ""
	}

	policyContext := engine.PolicyContext{
		NewResource:   newR,
		OldResource:   oldR,
		Context:       ctx,
		AdmissionInfo: userRequestInfo,
	}

	var engineResponses []response.EngineResponse
	for _, policy := range policies {
		logger.V(3).Info("evaluating policy", "policy", policy.Name)
		policyContext.Policy = policy
		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}
		engineResponses = append(engineResponses, engineResponse)
		ws.statusListener.Send(validateStats{
			resp: engineResponse,
		})
		if !engineResponse.IsSuccesful() {
			logger.V(4).Info("failed to apply policy", "policy", policy.Name, "failed rules", engineResponse.GetFailedRules())
			continue
		}

		logger.Info("valiadtion rules from policy applied succesfully", "policy", policy.Name)
	}
	// If Validation fails then reject the request
	// no violations will be created on "enforce"
	blocked := toBlockResource(engineResponses, logger)

	// REPORTING EVENTS
	// Scenario 1:
	//   resource is blocked, as there is a policy in "enforce" mode that failed.
	//   create an event on the policy to inform the resource request was blocked
	// Scenario 2:
	//   some/all policies failed to apply on the resource. a policy volation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 3:
	//   all policies were applied succesfully.
	//   create an event on the resource
	events := generateEvents(engineResponses, blocked, (request.Operation == v1beta1.Update), logger)
	ws.eventGen.Add(events...)
	if blocked {
		logger.V(4).Info("resource blocked")
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	// ADD POLICY VIOLATIONS
	// violations are created with resource on "audit"
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses, logger)
	ws.pvGenerator.Add(pvInfos...)
	// report time end
	return true, ""
}

type validateStats struct {
	resp response.EngineResponse
}

func (vs validateStats) PolicyName() string {
	return vs.resp.PolicyResponse.Policy
}

func (vs validateStats) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {
	if reflect.DeepEqual(response.EngineResponse{}, vs.resp) {
		return status
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range status.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range vs.resp.PolicyResponse.Rules {
		ruleStat := nameToRule[rule.Name]
		ruleStat.Name = rule.Name

		averageOver := int64(ruleStat.AppliedCount + ruleStat.FailedCount)
		ruleStat.ExecutionTime = updateAverageTime(
			rule.ProcessingTime,
			ruleStat.ExecutionTime,
			averageOver).String()

		if rule.Success {
			status.RulesAppliedCount++
			ruleStat.AppliedCount++
		} else {
			status.RulesFailedCount++
			ruleStat.FailedCount++
			if vs.resp.PolicyResponse.ValidationFailureAction == "enforce" {
				status.ResourcesBlockedCount++
				ruleStat.ResourcesBlockedCount++
			}
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
