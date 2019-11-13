package webhooks

import (
	"reflect"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	engine "github.com/nirmata/kyverno/pkg/engine"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

// handleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest,
	policies []*v1alpha1.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	glog.V(4).Infof("Receive request in validating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	var policyStats []policyctr.PolicyStat

	// gather stats from the engine response
	gatherStat := func(policyName string, policyResponse engine.PolicyResponse) {
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
	newR, oldR, err := extractResources(request)
	if err != nil {
		// as resource cannot be parsed, we skip processing
		glog.Error(err)
		return true, ""
	}

	policyContext := engine.PolicyContext{
		NewResource: newR,
		OldResource: oldR,
		AdmissionInfo: engine.RequestInfo{
			Roles:             roles,
			ClusterRoles:      clusterRoles,
			AdmissionUserInfo: request.UserInfo},
	}

	var engineResponses []engine.EngineResponse
	for _, policy := range policies {
		policyContext.Policy = *policy
		if !utils.ContainsString(getApplicableKindsForPolicy(policy), request.Kind.Kind) {
			continue
		}

		glog.V(2).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			newR.GetKind(), newR.GetNamespace(), newR.GetName(), request.UID, request.Operation)

		engineResponse := engine.Validate(policyContext)
		if reflect.DeepEqual(engineResponse, engine.EngineResponse{}) {
			// we get an empty response if old and new resources created the same response
			// allow updates if resource update doesnt change the policy evaluation
			continue
		}
		engineResponses = append(engineResponses, engineResponse)
		// Gather policy application statistics
		gatherStat(policy.Name, engineResponse.PolicyResponse)
		if !engineResponse.IsSuccesful() {
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s\n", policy.Name, newR.GetNamespace(), newR.GetName())
			continue
		}
	}
	// ADD EVENTS
	events := generateEvents(engineResponses, (request.Operation == v1beta1.Update))
	ws.eventGen.Add(events...)

	// If Validation fails then reject the request
	// violations are created with resource owner(if exist) on "enforce"
	// and if there are any then we dont block the resource creation
	// Even if one the policy being applied
	if !isResponseSuccesful(engineResponses) && toBlockResource(engineResponses) {
		policyviolation.CreatePVWhenBlocked(ws.pvLister, ws.kyvernoClient, ws.client, engineResponses)
		sendStat(true)
		return false, getErrorMsg(engineResponses)
	}

	// ADD POLICY VIOLATIONS
	// violations are created with resource on "audit"
	policyviolation.CreatePV(ws.pvLister, ws.kyvernoClient, engineResponses)
	sendStat(false)
	return true, ""
}
