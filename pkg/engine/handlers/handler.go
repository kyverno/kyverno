package handlers

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Handler interface {
	Process(
		context.Context,
		logr.Logger,
		engineapi.PolicyContext,
		unstructured.Unstructured,
		kyvernov1.Rule,
		engineapi.EngineContextLoader,
		[]kyvernov2beta1.PolicyException,
	) (unstructured.Unstructured, []engineapi.RuleResponse)
}

func WithError(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string, err error) []engineapi.RuleResponse {
	return WithResponses(engineapi.RuleError(rule.Name, ruleType, msg, err))
}

func WithSkip(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) []engineapi.RuleResponse {
	return WithResponses(engineapi.RuleSkip(rule.Name, ruleType, msg))
}

func WithPass(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) []engineapi.RuleResponse {
	return WithResponses(engineapi.RulePass(rule.Name, ruleType, msg))
}

func WithFail(rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) []engineapi.RuleResponse {
	return WithResponses(engineapi.RuleFail(rule.Name, ruleType, msg))
}

func WithResponses(rrs ...*engineapi.RuleResponse) []engineapi.RuleResponse {
	var out []engineapi.RuleResponse
	for _, rr := range rrs {
		if rr != nil {
			out = append(out, *rr)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
