package internal

import (
	"fmt"
	"reflect"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func RuleError(rule *kyvernov1.Rule, ruleType engineapi.RuleType, msg string, err error) *engineapi.RuleResponse {
	return RuleResponse(*rule, ruleType, fmt.Sprintf("%s: %s", msg, err.Error()), engineapi.RuleStatusError)
}

func RuleSkip(rule *kyvernov1.Rule, ruleType engineapi.RuleType, msg string) *engineapi.RuleResponse {
	return RuleResponse(*rule, ruleType, msg, engineapi.RuleStatusSkip)
}

func RulePass(rule *kyvernov1.Rule, ruleType engineapi.RuleType, msg string) *engineapi.RuleResponse {
	return RuleResponse(*rule, ruleType, msg, engineapi.RuleStatusPass)
}

func RuleResponse(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string, status engineapi.RuleStatus) *engineapi.RuleResponse {
	resp := &engineapi.RuleResponse{
		Name:    rule.Name,
		Type:    ruleType,
		Message: msg,
		Status:  status,
	}
	return resp
}

func AddRuleResponse(resp *engineapi.PolicyResponse, ruleResp *engineapi.RuleResponse, startTime time.Time) {
	ruleResp.Stats.ProcessingTime = time.Since(startTime)
	ruleResp.Stats.Timestamp = startTime.Unix()
	resp.Rules = append(resp.Rules, *ruleResp)
	if ruleResp.Status == engineapi.RuleStatusPass || ruleResp.Status == engineapi.RuleStatusFail {
		resp.Stats.RulesAppliedCount++
	} else if ruleResp.Status == engineapi.RuleStatusError {
		resp.Stats.RulesErrorCount++
	}
}

func BuildResponse(ctx engineapi.PolicyContext, resp *engineapi.EngineResponse, startTime time.Time) *engineapi.EngineResponse {
	if reflect.DeepEqual(resp.PatchedResource, unstructured.Unstructured{}) {
		// for delete requests patched resource will be oldResource since newResource is empty
		resource := ctx.NewResource()
		if reflect.DeepEqual(resource, unstructured.Unstructured{}) {
			resource = ctx.OldResource()
		}
		resp.PatchedResource = resource
	}
	resp.PolicyResponse.Stats.ProcessingTime = time.Since(startTime)
	resp.PolicyResponse.Stats.Timestamp = startTime.Unix()
	return resp
}
