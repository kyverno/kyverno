package policy

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPolicy applies policy on a resource
//TODO: generation rules
func applyPolicy(policy kyverno.Policy, resource unstructured.Unstructured, policyStatus PolicyStatusInterface) (info.PolicyInfo, error) {
	var ps PolicyStat
	gatherStat := func(policyName string, er engine.EngineResponse) {
		// ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.Stats.ValidationExecutionTime = er.ExecutionTime
		ps.Stats.RulesAppliedCount = er.RulesAppliedCount
	}
	// send stats for aggregation
	sendStat := func(blocked bool) {
		//SEND
		policyStatus.SendStat(ps)
	}

	startTime := time.Now()
	glog.V(4).Infof("Started apply policy %s on resource %s/%s/%s (%v)", policy.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), startTime)
	defer func() {
		glog.V(4).Infof("Finished applying %s on resource %s/%s/%s (%v)", policy.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), time.Since(startTime))
	}()
	// glog.V(4).Infof("apply policy %s with resource version %s on resource %s/%s/%s with resource version %s", policy.Name, policy.ResourceVersion, resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion())
	policyInfo := info.NewPolicyInfo(policy.Name, resource.GetKind(), resource.GetName(), resource.GetNamespace(), policy.Spec.ValidationFailureAction)

	//MUTATION
	mruleInfos, err := mutation(policy, resource, policyStatus)
	policyInfo.AddRuleInfos(mruleInfos)
	if err != nil {
		return policyInfo, err
	}

	//VALIDATION
	engineResponse := engine.Validate(policy, resource)
	if len(engineResponse.RuleInfos) != 0 {
		policyInfo.AddRuleInfos(engineResponse.RuleInfos)
	}
	// gather stats
	gatherStat(policy.Name, engineResponse)
	//send stats
	sendStat(false)

	//TODO: GENERATION
	return policyInfo, nil
}

func mutation(policy kyverno.Policy, resource unstructured.Unstructured, policyStatus PolicyStatusInterface) ([]info.RuleInfo, error) {
	var ps PolicyStat
	// gather stats from the engine response
	gatherStat := func(policyName string, er engine.EngineResponse) {
		// ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.Stats.MutationExecutionTime = er.ExecutionTime
		ps.Stats.RulesAppliedCount = er.RulesAppliedCount
	}
	// send stats for aggregation
	sendStat := func(blocked bool) {
		//SEND
		policyStatus.SendStat(ps)
	}

	engineResponse := engine.Mutate(policy, resource)
	// gather stats
	gatherStat(policy.Name, engineResponse)
	//send stats
	sendStat(false)

	patches := extractPatches(engineResponse)
	ruleInfos := engineResponse.RuleInfos
	if len(ruleInfos) == 0 {
		//no rules processed
		return nil, nil
	}

	for _, r := range ruleInfos {
		if !r.IsSuccessful() {
			// no failures while processing rule
			return ruleInfos, nil
		}
	}
	if len(patches) == 0 {
		// no patches for the resources
		// either there were failures or the overlay already was satisfied
		return ruleInfos, nil
	}

	// resources matches
	if reflect.DeepEqual(resource, engineResponse.PatchedResource) {
		ruleInfo := info.NewRuleInfo("over-all mutation", info.Mutation)
		ruleInfo.Add("resource satisfies the mutation rule")
		return append(ruleInfos, ruleInfo), nil
	}

	return getFailedOverallRuleInfo(resource, ruleInfos)
}

// getFailedOverallRuleInfo gets detailed info for over-all mutation failure
func getFailedOverallRuleInfo(resource unstructured.Unstructured, ruleInfos []info.RuleInfo) ([]info.RuleInfo, error) {
	ruleInfo := info.NewRuleInfo("over-all mutation", info.Mutation)

	rawResource, err := resource.MarshalJSON()
	if err != nil {
		glog.V(4).Infof("unable to marshal resource: %v\n", err)
		return ruleInfos, err
	}

	var failedRules []string

	// resource does not match so there was a mutation rule violated
	for _, ri := range ruleInfos {
		if len(ri.Patches) == 0 {
			continue
		}

		patch, err := jsonpatch.DecodePatch(utils.JoinPatches(ri.Patches))
		if err != nil {
			return ruleInfos, err
		}

		// apply the patches returned by mutate to the original resource
		patchedResource, err := patch.Apply(rawResource)
		if err != nil {
			return ruleInfos, err
		}

		if !jsonpatch.Equal(patchedResource, rawResource) {
			failedRules = append(failedRules, ri.Name)
		}
	}

	ruleInfo.Fail()
	ruleInfo.Add(fmt.Sprintf("rule %s might have failed", strings.Join(failedRules, ",")))
	return append(ruleInfos, ruleInfo), nil
}

// TODO: remove this once methods for engineResponse are implemented
func extractPatches(engineResponse engine.EngineResponse) [][]byte {
	var patches [][]byte
	for _, info := range engineResponse.RuleInfos {
		if len(info.Patches) != 0 {
			patches = append(patches, info.Patches...)
		}
	}
	return patches
}

// getRuleName gets the rule names from the index
func getRuleName(ruleInfos []info.RuleInfo, index int) string {
	var ruleNames []string

	for i := index; i < len(ruleInfos); i++ {
		ruleNames = append(ruleNames, ruleInfos[i].Name)
	}

	return strings.Join(ruleNames, ",")
}
