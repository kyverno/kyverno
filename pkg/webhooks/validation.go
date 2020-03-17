package webhooks

import (
	"reflect"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	logger := ws.log.WithValues("action", "validation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	logger.V(4).Info("incoming request")

	var policyStats []policyctr.PolicyStat
	// gather stats from the engine response
	gatherStat := func(policyName string, policyResponse response.PolicyResponse) {
		ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.Stats.ValidationExecutionTime = policyResponse.ProcessingTime
		ps.Stats.RulesAppliedCount = policyResponse.RulesAppliedCount
		// capture rule level stats
		for _, rule := range policyResponse.Rules {
			rs := policyctr.RuleStatinfo{}
			rs.RuleName = rule.Name
			rs.ExecutionTime = rule.RuleStats.ProcessingTime
			if rule.Success {
				rs.RuleAppliedCount++
			} else {
				rs.RulesFailedCount++
			}
			ps.Stats.Rules = append(ps.Stats.Rules, rs)
		}
		policyStats = append(policyStats, ps)
	}
	// send stats for aggregation
	sendStat := func(blocked bool) {
		for _, stat := range policyStats {
			stat.Stats.ResourceBlocked = utils.Btoi(blocked)
			//SEND
			ws.policyStatus.SendStat(stat)
		}
	}

	// Get new and old resource
	newR, oldR, err := extractResources(patchedResource, request)
	if err != nil {
		// as resource cannot be parsed, we skip processing
		logger.Error(err, "failed to extract resource")
		return true, ""
	}
	userRequestInfo := kyverno.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}
	// build context
	ctx := context.NewContext()
	// load incoming resource into the context
	err = ctx.AddResource(request.Object.Raw)
	if err != nil {
		logger.Error(err, "failed to load incoming resource in context")
	}

	err = ctx.AddUserInfo(userRequestInfo)
	if err != nil {
		logger.Error(err, "failed to load userInfo in context")
	}

	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load service account in context")
	}

	policyContext := engine.PolicyContext{
		NewResource:   newR,
		OldResource:   oldR,
		Context:       ctx,
		AdmissionInfo: userRequestInfo,
	}
	var engineResponses []response.EngineResponse
	for _, policy := range policies {
		logger.V(2).Info("evaluating policy", "policy", policy.Name)
		policyContext.Policy = policy
		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, response.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}
		engineResponses = append(engineResponses, engineResponse)
		// Gather policy application statistics
		gatherStat(policy.Name, engineResponse.PolicyResponse)
		if !engineResponse.IsSuccesful() {
			logger.V(4).Info("failed to apply policy", "policy", policy.Name)
			continue
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
	//   some/all policies failed to apply on the resource. a policy volation is generated.
	//   create an event on the resource and the policy that failed
	// Scenario 3:
	//   all policies were applied succesfully.
	//   create an event on the resource
	events := generateEvents(engineResponses, blocked, (request.Operation == v1beta1.Update), logger)
	ws.eventGen.Add(events...)
	if blocked {
		logger.V(4).Info("resource blocked")
		sendStat(true)
		// EVENTS
		// - event on the Policy
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	// ADD POLICY VIOLATIONS
	// violations are created with resource on "audit"
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses, logger)
	ws.pvGenerator.Add(pvInfos...)
	sendStat(false)
	// report time end
	return true, ""
}
