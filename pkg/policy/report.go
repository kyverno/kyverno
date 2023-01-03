package policy

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
)

func generateSuccessEvents(log logr.Logger, ers []*api.EngineResponse) (eventInfos []event.Info) {
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

func generateFailEvents(log logr.Logger, ers []*api.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		eventInfos = append(eventInfos, generateFailEventsPerEr(log, er)...)
	}
	return eventInfos
}

func generateFailEventsPerEr(log logr.Logger, er *api.EngineResponse) []event.Info {
	var eventInfos []event.Info
	logger := log.WithValues("policy", er.PolicyResponse.Policy.Name,
		"kind", er.PolicyResponse.Resource.Kind, "namespace", er.PolicyResponse.Resource.Namespace,
		"name", er.PolicyResponse.Resource.Name)

	for i, rule := range er.PolicyResponse.Rules {
		if rule.Status != api.RuleStatusPass && rule.Status != api.RuleStatusSkip {
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
