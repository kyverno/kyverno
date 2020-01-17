package webhooks

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	engineutils "github.com/nirmata/kyverno/pkg/engine/utils"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest, resource unstructured.Unstructured, policies []kyverno.ClusterPolicy, roles, clusterRoles []string) (bool, []byte, string) {
	glog.V(4).Infof("Receive request in mutating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	var patches [][]byte
	var policyStats []policyctr.PolicyStat

	// gather stats from the engine response
	gatherStat := func(policyName string, policyResponse response.PolicyResponse) {
		ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.Stats.MutationExecutionTime = policyResponse.ProcessingTime
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
			if rule.Patches != nil {
				rs.MutationCount++
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

	var engineResponses []response.EngineResponse

	userRequestInfo := kyverno.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: request.UserInfo}

	// build context
	ctx := context.NewContext()
	// load incoming resource into the context
	ctx.AddResource(request.Object.Raw)
	ctx.AddUserInfo(userRequestInfo)
	ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)

	policyContext := engine.PolicyContext{
		NewResource:   resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
	}

	for _, policy := range policies {
		glog.V(2).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)

		policyContext.Policy = policy
		// TODO: this can be
		engineResponse := engine.Mutate(policyContext)
		engineResponses = append(engineResponses, engineResponse)
		// Gather policy application statistics
		gatherStat(policy.Name, engineResponse.PolicyResponse)
		if !engineResponse.IsSuccesful() {
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s\n", policy.Name, resource.GetNamespace(), resource.GetName())
			continue
		}
		// gather patches
		patches = append(patches, engineResponse.GetPatches()...)
		glog.V(4).Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, resource.GetNamespace(), resource.GetName())
	}

	// generate annotations
	if annPatches := generateAnnotationPatches(engineResponses); annPatches != nil {
		patches = append(patches, annPatches)
	}

	// generate violation when referenced path does not exist
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses)
	ws.pvGenerator.Add(pvInfos...)

	// ADD EVENTS
	events := generateEvents(engineResponses, (request.Operation == v1beta1.Update))
	ws.eventGen.Add(events...)

	if isResponseSuccesful(engineResponses) {
		sendStat(false)
		patch := engineutils.JoinPatches(patches)
		return true, patch, ""
	}

	sendStat(true)
	glog.Errorf("Failed to mutate the resource, %s\n", getErrorMsg(engineResponses))
	return false, nil, getErrorMsg(engineResponses)
}
