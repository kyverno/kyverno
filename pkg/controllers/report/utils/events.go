package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"k8s.io/api/admissionregistration/v1alpha1"
)

func GenerateEvents(logger logr.Logger, eventGen event.Interface, config config.Configuration, results ...engineapi.EngineResponse) {
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

func generateSuccessEvents(log logr.Logger, ers ...engineapi.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		pol := er.Policy()
		polType := pol.GetType() 
		if polType == engineapi.ValidatingAdmissionPolicyType {
			vap := pol.GetPolicy().(v1alpha1.ValidatingAdmissionPolicy)
			logger := log.WithValues("policy", vap.GetName(), "kind", er.Resource.GetKind(), "namespace", er.Resource.GetNamespace(), "name", er.Resource.GetName())
			if !er.IsFailed() {
				logger.V(4).Info("generating event on policy for success rules")
				e := event.NewPolicyAppliedEvent(event.PolicyController, er)
				eventInfos = append(eventInfos, e)
			}
		} else {
			kyvernopol := pol.GetPolicy().(kyvernov1.PolicyInterface)
			logger := log.WithValues("policy", kyvernopol.GetName(), "kind", er.Resource.GetKind(), "namespace", er.Resource.GetNamespace(), "name", er.Resource.GetName())
			if !er.IsFailed() {
				logger.V(4).Info("generating event on policy for success rules")
				e := event.NewPolicyAppliedEvent(event.PolicyController, er)
				eventInfos = append(eventInfos, e)
			}
		}
	}
	return eventInfos
}

func generateExceptionEvents(log logr.Logger, ers ...engineapi.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		for _, ruleResp := range er.PolicyResponse.Rules {
			if ruleResp.Status() == engineapi.RuleStatusSkip && ruleResp.IsException() {
				eventInfos = append(eventInfos, event.NewPolicyExceptionEvents(er, ruleResp, event.PolicyController)...)
			}
		}
	}
	return eventInfos
}

func generateFailEvents(log logr.Logger, ers ...engineapi.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		eventInfos = append(eventInfos, generateFailEventsPerEr(log, er)...)
	}
	return eventInfos
}

func generateFailEventsPerEr(log logr.Logger, er engineapi.EngineResponse) []event.Info {
	var eventInfos []event.Info
	pol := er.Policy()
	polType := pol.GetType()
	if polType == engineapi.ValidatingAdmissionPolicyType {
		vap := pol.GetPolicy().(v1alpha1.ValidatingAdmissionPolicy) 
		logger := log.WithValues(
			"policy", vap.GetName(),
			"kind", er.Resource.GetKind(),
			"namespace", er.Resource.GetNamespace(),
			"name", er.Resource.GetName(),
		)
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status() != engineapi.RuleStatusPass && rule.Status() != engineapi.RuleStatusSkip {
				eventResource := event.NewResourceViolationEvent(event.PolicyController, event.PolicyViolation, er, rule)
				eventInfos = append(eventInfos, eventResource)
				eventPolicy := event.NewPolicyFailEvent(event.PolicyController, event.PolicyViolation, er, rule, false)
				eventInfos = append(eventInfos, eventPolicy)
			}
		}
		if len(eventInfos) > 0 {
			logger.V(4).Info("generating events for policy", "events", eventInfos)
		}
	} else {
		kyvernopol := pol.GetPolicy().(kyvernov1.PolicyInterface)
		logger := log.WithValues(
			"policy", kyvernopol.GetName(),
			"kind", er.Resource.GetKind(),
			"namespace", er.Resource.GetNamespace(),
			"name", er.Resource.GetName(),
		)
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status() != engineapi.RuleStatusPass && rule.Status() != engineapi.RuleStatusSkip {
				eventResource := event.NewResourceViolationEvent(event.PolicyController, event.PolicyViolation, er, rule)
				eventInfos = append(eventInfos, eventResource)
				eventPolicy := event.NewPolicyFailEvent(event.PolicyController, event.PolicyViolation, er, rule, false)
				eventInfos = append(eventInfos, eventPolicy)
			}
		}
		if len(eventInfos) > 0 {
			logger.V(4).Info("generating events for policy", "events", eventInfos)
		}
	}
	return eventInfos
}
