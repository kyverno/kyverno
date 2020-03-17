package webhooks

import (
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
// return value: generated patches
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest, resource unstructured.Unstructured, policies []kyverno.ClusterPolicy, roles, clusterRoles []string) []byte {
	logger := ws.log.WithValues("action", "mutation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	logger.V(4).Info("incoming request")

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
	var err error
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
		NewResource:   resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
	}

	for _, policy := range policies {
		logger.V(2).Info("evaluating policy", "policy", policy.Name)

		policyContext.Policy = policy
		engineResponse := engine.Mutate(policyContext)
		engineResponses = append(engineResponses, engineResponse)
		// Gather policy application statistics
		gatherStat(policy.Name, engineResponse.PolicyResponse)
		if !engineResponse.IsSuccesful() {
			logger.V(4).Info("failed to apply policy", "policy", policy.Name)
			continue
		}
		// gather patches
		patches = append(patches, engineResponse.GetPatches()...)
		logger.Info("mutation rules from policy applied succesfully", "policy", policy.Name)

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

	sendStat(false)

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
