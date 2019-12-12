package webhooks

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
)

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
// patchedResource is the (resource + patches) after applying mutation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	glog.V(4).Infof("Receive request in validating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	var policyStats []policyctr.PolicyStat
	evalTime := time.Now()
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
	newR, oldR, err := extractResources(request)
	if err != nil {
		// as resource cannot be parsed, we skip processing
		glog.Error(err)
		return true, ""
	}
	// build context
	ctx := context.NewContext()
	// load incoming resource into the context
	ctx.Add("resource", request.Object.Raw)
	ctx.Add("user", transformUser(request.UserInfo))
	policyContext := engine.PolicyContext{
		NewResource: newR,
		OldResource: oldR,
		Context:     ctx,
		AdmissionInfo: engine.RequestInfo{
			Roles:             roles,
			ClusterRoles:      clusterRoles,
			AdmissionUserInfo: request.UserInfo},
	}
	var engineResponses []response.EngineResponse
	for _, policy := range policies {
		glog.V(2).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			newR.GetKind(), newR.GetNamespace(), newR.GetName(), request.UID, request.Operation)
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
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s\n", policy.Name, newR.GetNamespace(), newR.GetName())
			continue
		}
	}
	glog.V(4).Infof("eval: %v %s/%s/%s ", time.Since(evalTime), request.Kind, request.Namespace, request.Name)
	// report time
	reportTime := time.Now()

	// If Validation fails then reject the request
	// violations are created with resource owner(if exist) on "enforce"
	// and if there are any then we dont block the resource creation
	// Even if one the policy being applied

	blocked := toBlockResource(engineResponses)
	if !isResponseSuccesful(engineResponses) && blocked {
		glog.V(4).Infof("resource %s/%s/%s is blocked\n", newR.GetKind(), newR.GetNamespace(), newR.GetName())
		pvInfos := generatePV(engineResponses, true)
		ws.pvGenerator.Add(pvInfos...)
		// ADD EVENTS
		events := generateEvents(engineResponses, (request.Operation == v1beta1.Update))
		ws.eventGen.Add(events...)
		sendStat(true)
		return false, getErrorMsg(engineResponses)
	}
	// ADD POLICY VIOLATIONS
	// violations are created with resource on "audit"

	pvInfos := generatePV(engineResponses, blocked)
	ws.pvGenerator.Add(pvInfos...)
	// ADD EVENTS
	events := generateEvents(engineResponses, (request.Operation == v1beta1.Update))
	ws.eventGen.Add(events...)
	sendStat(false)
	// report time end
	glog.V(4).Infof("report: %v %s/%s/%s", time.Since(reportTime), request.Kind, request.Namespace, request.Name)
	return true, ""
}

func transformUser(userInfo authenticationv1.UserInfo) []byte {
	data, err := json.Marshal(userInfo)
	if err != nil {
		glog.Errorf("failed to marshall resource %v: %v", userInfo, err)
		return nil
	}
	return data
}
