package policy

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policyreport"
)

func (pc *PolicyController) report(policy string, engineResponses []response.EngineResponse, logger logr.Logger) {
	eventInfos := generateEvents(logger, engineResponses)
	pc.eventGen.Add(eventInfos...)

	pvInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)

	// as engineResponses holds the results for all matched resources in one namespace
	// we can merge pvInfos into a single object to reduce update frequency (throttling request) on RCR
	info := mergePvInfos(pvInfos)
	pc.prGenerator.Add(info)
	logger.V(4).Info("added a request to RCR generator", "key", info.ToKey())
}

func generateEvents(log logr.Logger, ers []response.EngineResponse) []event.Info {
	var eventInfos []event.Info
	for _, er := range ers {
		if er.IsSuccessful() {
			continue
		}
		eventInfos = append(eventInfos, generateEventsPerEr(log, er)...)
	}
	return eventInfos
}

func generateEventsPerEr(log logr.Logger, er response.EngineResponse) []event.Info {
	var eventInfos []event.Info

	logger := log.WithValues("policy", er.PolicyResponse.Policy, "kind", er.PolicyResponse.Resource.Kind, "namespace", er.PolicyResponse.Resource.Namespace, "name", er.PolicyResponse.Resource.Name)
	logger.V(4).Info("reporting results for policy")

	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		// generate event on resource for each failed rule
		logger.V(4).Info("generating event on resource")
		e := event.Info{}
		e.Kind = er.PolicyResponse.Resource.Kind
		e.Namespace = er.PolicyResponse.Resource.Namespace
		e.Name = er.PolicyResponse.Resource.Name
		e.Reason = event.PolicyViolation.String()
		e.Source = event.PolicyController
		e.Message = fmt.Sprintf("policy '%s' (%s) rule '%s' failed. %v", er.PolicyResponse.Policy, rule.Type, rule.Name, rule.Message)
		eventInfos = append(eventInfos, e)
	}

	return eventInfos
}

func mergePvInfos(infos []policyreport.Info) policyreport.Info {
	aggregatedInfo := policyreport.Info{}
	if len(infos) == 0 {
		return aggregatedInfo
	}

	var results []policyreport.EngineResponseResult
	for _, info := range infos {
		for _, res := range info.Results {
			results = append(results, res)
		}
	}

	aggregatedInfo.PolicyName = infos[0].PolicyName
	aggregatedInfo.Namespace = infos[0].Namespace
	aggregatedInfo.Results = results
	return aggregatedInfo
}
