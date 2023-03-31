package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
	) (unstructured.Unstructured, []engineapi.RuleResponse)
}

func WithResponses(ruleResponses ...engineapi.RuleResponse) []engineapi.RuleResponse {
	if len(ruleResponses) == 0 {
		return nil
	}
	return ruleResponses
}

func WithError(timestamp time.Time, rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string, err error) []engineapi.RuleResponse {
	return WithResponses(engineapi.RuleError(timestamp, rule, ruleType, msg, err).DoneNow())
}

func WithSkip(timestamp time.Time, rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) []engineapi.RuleResponse {
	return WithResponses(engineapi.RuleSkip(timestamp, rule, ruleType, msg).DoneNow())
}

func WithPass(timestamp time.Time, rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) []engineapi.RuleResponse {
	return WithResponses(engineapi.RulePass(timestamp, rule, ruleType, msg).DoneNow())
}

func WithFail(timestamp time.Time, rule kyvernov1.Rule, ruleType engineapi.RuleType, msg string) []engineapi.RuleResponse {
	return WithResponses(engineapi.RuleFail(timestamp, rule, ruleType, msg).DoneNow())
type HandlerFactory = func() (Handler, error)

func WithHandler(handler Handler) HandlerFactory {
	return func() (Handler, error) {
		return handler, nil
	}
}

func RuleResponses(rrs ...*engineapi.RuleResponse) []engineapi.RuleResponse {
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
