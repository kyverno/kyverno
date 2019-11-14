package policy

import (
	"fmt"
	"reflect"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPolicy applies policy on a resource
//TODO: generation rules
func applyPolicy(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, policyStatus PolicyStatusInterface) (responses []engine.EngineResponse) {
	startTime := time.Now()
	var policyStats []PolicyStat
	glog.V(4).Infof("Started apply policy %s on resource %s/%s/%s (%v)", policy.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), startTime)
	defer func() {
		glog.V(4).Infof("Finished applying %s on resource %s/%s/%s (%v)", policy.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), time.Since(startTime))
	}()

	// gather stats from the engine response
	gatherStat := func(policyName string, policyResponse engine.PolicyResponse) {
		ps := PolicyStat{}
		ps.PolicyName = policyName
		ps.Stats.MutationExecutionTime = policyResponse.ProcessingTime
		ps.Stats.RulesAppliedCount = policyResponse.RulesAppliedCount
		// capture rule level stats
		for _, rule := range policyResponse.Rules {
			rs := RuleStatinfo{}
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
			policyStatus.SendStat(stat)
		}
	}
	var engineResponses []engine.EngineResponse
	var engineResponse engine.EngineResponse
	var err error

	//MUTATION
	engineResponse, err = mutation(policy, resource, policyStatus)
	engineResponses = append(engineResponses, engineResponse)
	if err != nil {
		glog.Errorf("unable to process mutation rules: %v", err)
	}
	gatherStat(policy.Name, engineResponse.PolicyResponse)
	//send stats
	sendStat(false)

	//VALIDATION
	engineResponse = engine.Validate(engine.PolicyContext{Policy: policy, NewResource: resource})
	engineResponses = append(engineResponses, engineResponse)
	// gather stats
	gatherStat(policy.Name, engineResponse.PolicyResponse)
	//send stats
	sendStat(false)

	//TODO: GENERATION
	return engineResponses
}
func mutation(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, policyStatus PolicyStatusInterface) (engine.EngineResponse, error) {
	engineResponse := engine.Mutate(engine.PolicyContext{Policy: policy, NewResource: resource})
	if !engineResponse.IsSuccesful() {
		glog.V(4).Infof("mutation had errors reporting them")
		return engineResponse, nil
	}
	// Verify if the JSON pathes returned by the Mutate are already applied to the resource
	if reflect.DeepEqual(resource, engineResponse.PatchedResource) {
		// resources matches
		glog.V(4).Infof("resource %s/%s/%s satisfies policy %s", engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Namespace, engineResponse.PolicyResponse.Resource.Name, engineResponse.PolicyResponse.Policy)
		return engineResponse, nil
	}
	return getFailedOverallRuleInfo(resource, engineResponse)
}

// getFailedOverallRuleInfo gets detailed info for over-all mutation failure
func getFailedOverallRuleInfo(resource unstructured.Unstructured, engineResponse engine.EngineResponse) (engine.EngineResponse, error) {
	rawResource, err := resource.MarshalJSON()
	if err != nil {
		glog.V(4).Infof("unable to marshal resource: %v\n", err)
		return engine.EngineResponse{}, err
	}

	// resource does not match so there was a mutation rule violated
	for index, rule := range engineResponse.PolicyResponse.Rules {
		glog.V(4).Infof("veriying if policy %s rule %s was applied before to resource %s/%s/%s", engineResponse.PolicyResponse.Policy, rule.Name, engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Namespace, engineResponse.PolicyResponse.Resource.Name)
		if len(rule.Patches) == 0 {
			continue
		}

		patch, err := jsonpatch.DecodePatch(utils.JoinPatches(rule.Patches))
		if err != nil {
			glog.V(4).Infof("unable to decode patch %s: %v", rule.Patches, err)
			return engine.EngineResponse{}, err
		}

		// apply the patches returned by mutate to the original resource
		patchedResource, err := patch.Apply(rawResource)
		if err != nil {
			glog.V(4).Infof("unable to apply patch %s: %v", rule.Patches, err)
			return engine.EngineResponse{}, err
		}

		if !jsonpatch.Equal(patchedResource, rawResource) {
			glog.V(4).Infof("policy %s rule %s condition not satisifed by existing resource", engineResponse.PolicyResponse.Policy, rule.Name)
			engineResponse.PolicyResponse.Rules[index].Success = false
			engineResponse.PolicyResponse.Rules[index].Message = fmt.Sprintf("rule not satisfied by existing resource.")
		}
	}
	return engineResponse, nil
}
