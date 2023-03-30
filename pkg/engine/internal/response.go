package internal

import (
	"fmt"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func RuleError(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string, err error) *engineapi.RuleResponse {
	return RuleResponse(rule, ruleType, fmt.Sprintf("%s: %s", msg, err.Error()), engineapi.RuleStatusError)
}

func RuleSkip(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) *engineapi.RuleResponse {
	return RuleResponse(rule, ruleType, msg, engineapi.RuleStatusSkip)
}

func RulePass(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) *engineapi.RuleResponse {
	return RuleResponse(rule, ruleType, msg, engineapi.RuleStatusPass)
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

func BuildResponse(ctx engineapi.PolicyContext, resp *engineapi.EngineResponse, startTime time.Time) *engineapi.EngineResponse {
	if resp.PatchedResource.Object == nil {
		// for delete requests patched resource will be oldResource since newResource is empty
		resource := ctx.NewResource()
		if resource.Object == nil {
			resource = ctx.OldResource()
		}
		resp.PatchedResource = resource
	}
	resp.Stats.ProcessingTime = time.Since(startTime)
	resp.Stats.Timestamp = startTime.Unix()
	return resp
}
