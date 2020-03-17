package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyPolicy applies policy on a resource
//TODO: generation rules
func applyPolicy(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, policyStatus PolicyStatusInterface, log logr.Logger) (responses []response.EngineResponse) {
	logger := log.WithValues("kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	startTime := time.Now()
	var policyStats []PolicyStat
	logger.Info("start applying policy", "startTime", startTime)
	defer func() {
		logger.Info("finisnhed applying policy", "processingTime", time.Since(startTime))
	}()

	// gather stats from the engine response
	gatherStat := func(policyName string, policyResponse response.PolicyResponse) {
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
	var engineResponses []response.EngineResponse
	var engineResponse response.EngineResponse
	var err error
	// build context
	ctx := context.NewContext()
	ctx.AddResource(transformResource(resource))

	//MUTATION
	engineResponse, err = mutation(policy, resource, policyStatus, ctx, logger)
	engineResponses = append(engineResponses, engineResponse)
	if err != nil {
		logger.Error(err, "failed to process mutation rule")
	}
	gatherStat(policy.Name, engineResponse.PolicyResponse)
	//send stats
	sendStat(false)

	//VALIDATION
	engineResponse = engine.Validate(engine.PolicyContext{Policy: policy, Context: ctx, NewResource: resource})
	engineResponses = append(engineResponses, engineResponse)
	// gather stats
	gatherStat(policy.Name, engineResponse.PolicyResponse)
	//send stats
	sendStat(false)

	//TODO: GENERATION
	return engineResponses
}
func mutation(policy kyverno.ClusterPolicy, resource unstructured.Unstructured, policyStatus PolicyStatusInterface, ctx context.EvalInterface, log logr.Logger) (response.EngineResponse, error) {

	engineResponse := engine.Mutate(engine.PolicyContext{Policy: policy, NewResource: resource, Context: ctx})
	if !engineResponse.IsSuccesful() {
		log.V(4).Info("failed to apply mutation rules; reporting them")
		return engineResponse, nil
	}
	// Verify if the JSON pathes returned by the Mutate are already applied to the resource
	if reflect.DeepEqual(resource, engineResponse.PatchedResource) {
		// resources matches
		log.V(4).Info("resource already satisfys the policy")
		return engineResponse, nil
	}
	return getFailedOverallRuleInfo(resource, engineResponse, log)
}

// getFailedOverallRuleInfo gets detailed info for over-all mutation failure
func getFailedOverallRuleInfo(resource unstructured.Unstructured, engineResponse response.EngineResponse, log logr.Logger) (response.EngineResponse, error) {
	rawResource, err := resource.MarshalJSON()
	if err != nil {
		log.Error(err, "faield to marshall resource")
		return response.EngineResponse{}, err
	}

	// resource does not match so there was a mutation rule violated
	for index, rule := range engineResponse.PolicyResponse.Rules {
		log.V(4).Info("veriying if policy rule was applied before", "rule", rule.Name)
		if len(rule.Patches) == 0 {
			continue
		}
		patch, err := jsonpatch.DecodePatch(utils.JoinPatches(rule.Patches))
		if err != nil {
			log.Error(err, "failed to decode JSON patch", "patches", rule.Patches)
			return response.EngineResponse{}, err
		}

		// apply the patches returned by mutate to the original resource
		patchedResource, err := patch.Apply(rawResource)
		if err != nil {
			log.Error(err, "failed to apply JSON patch", "patches", rule.Patches)
			return response.EngineResponse{}, err
		}
		if !jsonpatch.Equal(patchedResource, rawResource) {
			log.V(4).Info("policy rule conditions not satisfied by resource", "rule", rule.Name)
			engineResponse.PolicyResponse.Rules[index].Success = false
			engineResponse.PolicyResponse.Rules[index].Message = fmt.Sprintf("mutation json patches not found at resource path %s", extractPatchPath(rule.Patches, log))
		}
	}
	return engineResponse, nil
}

type jsonPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func extractPatchPath(patches [][]byte, log logr.Logger) string {
	var resultPath []string
	// extract the patch path and value
	for _, patch := range patches {
		log.V(4).Info("expected json patch not found in resource", "patch", string(patch))
		var data jsonPatch
		if err := json.Unmarshal(patch, &data); err != nil {
			log.Error(err, "failed to decode the generate patch", "patch", string(patch))
			continue
		}
		resultPath = append(resultPath, data.Path)
	}
	return strings.Join(resultPath, ";")
}
