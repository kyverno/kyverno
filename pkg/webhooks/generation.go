package webhooks

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/config"
	"reflect"
	"sort"
	"strings"
	"time"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/webhooks/generate"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//HandleGenerate handles admission-requests for policies with generate rules
func (ws *WebhookServer) HandleGenerate(request *v1beta1.AdmissionRequest, policies []*kyverno.ClusterPolicy, ctx *context.Context, userRequestInfo kyverno.RequestInfo,dynamicConfig config.Interface) {
	logger := ws.log.WithValues("action", "generation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	logger.V(4).Info("incoming request")
	var engineResponses []response.EngineResponse

	if len(policies) == 0 {
		return
	}
	// convert RAW to unstructured
	resource, err := utils.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		//TODO: skip applying the admission control ?
		logger.Error(err, "failed to convert RAR resource to unstructured format")
		return
	}

	// CREATE resources, do not have name, assigned in admission-request

	policyContext := engine.PolicyContext{
		NewResource:   *resource,
		AdmissionInfo: userRequestInfo,
		Context:       ctx,
		ExcludeGroupRole : dynamicConfig.GetExcludeGroupRole(),
	}

	// engine.Generate returns a list of rules that are applicable on this resource
	for _, policy := range policies {
		policyContext.Policy = *policy
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
	if failedResponse := applyGenerateRequest(ws.grGenerator, userRequestInfo, request.Operation, engineResponses...); err != nil {
		// report failure event
		for _, failedGR := range failedResponse {
			events := failedEvents(fmt.Errorf("failed to create Generate Request: %v", failedGR.err), failedGR.gr, *resource)
			ws.eventGen.Add(events...)
		}
	}

	// Generate Stats wont be used here, as we delegate the generate rule
	// - Filter policies that apply on this resource
	// - - build CR context(userInfo+roles+clusterRoles)
	// - Create CR
	// - send Success
	// HandleGeneration  always returns success

	// Filter Policies
	return
}

func applyGenerateRequest(gnGenerator generate.GenerateRequests, userRequestInfo kyverno.RequestInfo,
	action v1beta1.Operation, engineResponses ...response.EngineResponse) (failedGenerateRequest []generateRequestResponse) {

	for _, er := range engineResponses {
		gr := transform(userRequestInfo, er)
		if err := gnGenerator.Apply(gr, action); err != nil {
			failedGenerateRequest = append(failedGenerateRequest, generateRequestResponse{gr: gr, err: err})
		}
	}
	return
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

type generateRequestResponse struct {
	gr  v1.GenerateRequestSpec
	err error
}

func (resp generateRequestResponse) info() string {
	return strings.Join([]string{resp.gr.Resource.Kind, resp.gr.Resource.Namespace, resp.gr.Resource.Name}, "/")
}

func (resp generateRequestResponse) error() string {
	return resp.err.Error()
}

func failedEvents(err error, gr kyverno.GenerateRequestSpec, resource unstructured.Unstructured) []event.Info {
	re := event.Info{}
	re.Kind = resource.GetKind()
	re.Namespace = resource.GetNamespace()
	re.Name = resource.GetName()
	re.Reason = event.PolicyFailed.String()
	re.Source = event.GeneratePolicyController
	re.Message = fmt.Sprintf("policy %s failed to apply: %v", gr.Policy, err)

	return []event.Info{re}
}
