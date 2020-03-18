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
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

//HandleGenerate handles admission-requests for policies with generate rules
func (ws *WebhookServer) HandleGenerate(request *v1beta1.AdmissionRequest, policies []kyverno.ClusterPolicy, patchedResource []byte, roles, clusterRoles []string) (bool, string) {
	logger := ws.log.WithValues("action", "generation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	logger.V(4).Info("incoming request")
	var engineResponses []response.EngineResponse

	// convert RAW to unstructured
	resource, err := utils.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		//TODO: skip applying the admission control ?
		logger.Error(err, "failed to convert RAR resource to unstructured format")
		return true, ""
	}

	// CREATE resources, do not have name, assigned in admission-request

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
	// load service account in context
	err = ctx.AddSA(userRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load service account in context")
	}

	policyContext := engine.PolicyContext{
		NewResource:   *resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
	}

	// engine.Generate returns a list of rules that are applicable on this resource
	for _, policy := range policies {
		policyContext.Policy = policy
		engineResponse := engine.Generate(policyContext)
		if len(engineResponse.PolicyResponse.Rules) > 0 {
			// some generate rules do apply to the resource
			engineResponses = append(engineResponses, engineResponse)
			ws.statusListener.Send(generateStats{
				resp: engineResponse,
			})
		}
	}
	// Adds Generate Request to a channel(queue size 1000) to generators
	if err := createGenerateRequest(ws.grGenerator, userRequestInfo, engineResponses...); err != nil {
		//TODO: send appropriate error
		return false, "Kyverno blocked: failed to create Generate Requests"
	}
	// Generate Stats wont be used here, as we delegate the generate rule
	// - Filter policies that apply on this resource
	// - - build CR context(userInfo+roles+clusterRoles)
	// - Create CR
	// - send Success
	// HandleGeneration  always returns success

	// Filter Policies
	return true, ""
}

func createGenerateRequest(gnGenerator generate.GenerateRequests, userRequestInfo kyverno.RequestInfo, engineResponses ...response.EngineResponse) error {
	for _, er := range engineResponses {
		if err := gnGenerator.Create(transform(userRequestInfo, er)); err != nil {
			return err
		}
	}
	return nil
}

func transform(userRequestInfo kyverno.RequestInfo, er response.EngineResponse) kyverno.GenerateRequestSpec {
	gr := kyverno.GenerateRequestSpec{
		Policy: er.PolicyResponse.Policy,
		Resource: kyverno.ResourceSpec{
			Kind:      er.PolicyResponse.Resource.Kind,
			Namespace: er.PolicyResponse.Resource.Namespace,
			Name:      er.PolicyResponse.Resource.Name,
		},
		Context: kyverno.GenerateRequestContext{
			UserRequestInfo: userRequestInfo,
		},
	}
	return gr
}

type generateStats struct {
	resp response.EngineResponse
}

func (gs generateStats) PolicyName() string {
	return gs.resp.PolicyResponse.Policy
}

func (gs generateStats) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {
	if reflect.DeepEqual(response.EngineResponse{}, gs.resp) {
		return status
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range status.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range gs.resp.PolicyResponse.Rules {
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

func updateAverageTime(newTime time.Duration, oldAverageTimeString string, averageOver int64) time.Duration {
	if averageOver == 0 {
		return newTime
	}
	oldAverageExecutionTime, _ := time.ParseDuration(oldAverageTimeString)
	numerator := (oldAverageExecutionTime.Nanoseconds() * averageOver) + newTime.Nanoseconds()
	denominator := averageOver + 1
	newAverageTimeInNanoSeconds := numerator / denominator
	return time.Duration(newAverageTimeInNanoSeconds) * time.Nanosecond
}
