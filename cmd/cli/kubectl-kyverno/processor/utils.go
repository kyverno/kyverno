package processor

import (
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func combineRuleResponses(imageResponse engineapi.EngineResponse) engineapi.EngineResponse {
	if imageResponse.PolicyResponse.RulesAppliedCount() == 0 {
		return imageResponse
	}

	completeRuleResponses := imageResponse.PolicyResponse.Rules
	var combineRuleResponses []engineapi.RuleResponse

	ruleNameType := make(map[string][]engineapi.RuleResponse)
	for _, rsp := range completeRuleResponses {
		key := rsp.Name() + ";" + string(rsp.RuleType())
		ruleNameType[key] = append(ruleNameType[key], rsp)
	}

	for key, ruleResponses := range ruleNameType {
		tokens := strings.Split(key, ";")
		ruleName := tokens[0]
		ruleType := tokens[1]
		var failRuleResponses []engineapi.RuleResponse
		var errorRuleResponses []engineapi.RuleResponse
		var passRuleResponses []engineapi.RuleResponse
		var skipRuleResponses []engineapi.RuleResponse

		ruleMesssage := ""
		for _, rsp := range ruleResponses {
			if rsp.Status() == engineapi.RuleStatusFail {
				failRuleResponses = append(failRuleResponses, rsp)
			} else if rsp.Status() == engineapi.RuleStatusError {
				errorRuleResponses = append(errorRuleResponses, rsp)
			} else if rsp.Status() == engineapi.RuleStatusPass {
				passRuleResponses = append(passRuleResponses, rsp)
			} else if rsp.Status() == engineapi.RuleStatusSkip {
				skipRuleResponses = append(skipRuleResponses, rsp)
			}
		}
		if len(errorRuleResponses) > 0 {
			for _, errRsp := range errorRuleResponses {
				ruleMesssage += errRsp.Message() + ";"
			}
			errorResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusError)
			combineRuleResponses = append(combineRuleResponses, *errorResponse)
			continue
		}

		if len(failRuleResponses) > 0 {
			for _, failRsp := range failRuleResponses {
				ruleMesssage += failRsp.Message() + ";"
			}
			failResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusFail)
			combineRuleResponses = append(combineRuleResponses, *failResponse)
			continue
		}

		if len(passRuleResponses) > 0 {
			for _, passRsp := range passRuleResponses {
				ruleMesssage += passRsp.Message() + ";"
			}
			passResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusPass)
			combineRuleResponses = append(combineRuleResponses, *passResponse)
			continue
		}

		for _, skipRsp := range skipRuleResponses {
			ruleMesssage += skipRsp.Message() + ";"
		}
		skipResponse := engineapi.NewRuleResponse(ruleName, engineapi.RuleType(ruleType), ruleMesssage, engineapi.RuleStatusSkip)
		combineRuleResponses = append(combineRuleResponses, *skipResponse)
	}
	imageResponse.PolicyResponse.Rules = combineRuleResponses
	return imageResponse
}

func needsNamespaceLabels(rules ...kyvernov1.Rule) bool {
	for _, p := range rules {
		if p.MatchResources.ResourceDescription.NamespaceSelector != nil ||
			p.ExcludeResources.ResourceDescription.NamespaceSelector != nil {
			return true
		}
		for _, m := range p.MatchResources.Any {
			if m.ResourceDescription.NamespaceSelector != nil {
				return true
			}
		}
		for _, m := range p.MatchResources.All {
			if m.ResourceDescription.NamespaceSelector != nil {
				return true
			}
		}
		for _, e := range p.ExcludeResources.Any {
			if e.ResourceDescription.NamespaceSelector != nil {
				return true
			}
		}
		for _, e := range p.ExcludeResources.All {
			if e.ResourceDescription.NamespaceSelector != nil {
				return true
			}
		}
	}
	return false
}
