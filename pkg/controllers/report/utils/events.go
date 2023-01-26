package utils

import (
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
)

func GenerateEvents(logger logr.Logger, eventGen event.Interface, config config.Configuration, results ...*response.EngineResponse) {
	for _, result := range results {
		var eventInfos []event.Info
		eventInfos = append(eventInfos, generateFailEvents(logger, result)...)
		eventInfos = append(eventInfos, generateExceptionEvents(logger, result)...)
		if config.GetGenerateSuccessEvents() {
			eventInfos = append(eventInfos, generateSuccessEvents(logger, result)...)
		}
		eventGen.Add(eventInfos...)
	}
}

func generateSuccessEvents(log logr.Logger, ers ...*response.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		logger := log.WithValues("policy", er.PolicyResponse.Policy, "kind", er.PolicyResponse.Resource.Kind, "namespace", er.PolicyResponse.Resource.Namespace, "name", er.PolicyResponse.Resource.Name)
		if !er.IsFailed() {
			logger.V(4).Info("generating event on policy for success rules")
			e := event.NewPolicyAppliedEvent(event.PolicyController, er)
			eventInfos = append(eventInfos, e)
		}
	}
	return eventInfos
}

func generateExceptionEvents(log logr.Logger, ers ...*response.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		for i, ruleResp := range er.PolicyResponse.Rules {
			isException := strings.Contains(ruleResp.Message, "rule skipped due to policy exception")
			if ruleResp.Status == response.RuleStatusSkip && isException {
				eventInfos = append(eventInfos, event.NewPolicyExceptionEvents(event.PolicyController, er, &er.PolicyResponse.Rules[i])...)
			}
		}
	}
	return eventInfos
}

func generateFailEvents(log logr.Logger, ers ...*response.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		eventInfos = append(eventInfos, generateFailEventsPerEr(log, er)...)
	}
	return eventInfos
}

func generateFailEventsPerEr(log logr.Logger, er *response.EngineResponse) []event.Info {
	var eventInfos []event.Info
	logger := log.WithValues(
		"policy", er.PolicyResponse.Policy.Name,
		"kind", er.PolicyResponse.Resource.Kind,
		"namespace", er.PolicyResponse.Resource.Namespace,
		"name", er.PolicyResponse.Resource.Name,
	)
	for i, rule := range er.PolicyResponse.Rules {
		if rule.Status != response.RuleStatusPass && rule.Status != response.RuleStatusSkip {
			eventResource := event.NewResourceViolationEvent(event.PolicyController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i])
			eventInfos = append(eventInfos, eventResource)
			eventPolicy := event.NewPolicyFailEvent(event.PolicyController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i], false)
			eventInfos = append(eventInfos, eventPolicy)
		}
	}
	if len(eventInfos) > 0 {
		logger.V(4).Info("generating events for policy", "events", eventInfos)
	}
	return eventInfos
}
