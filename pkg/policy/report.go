package policy

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyreport"
)

// for each policy-resource response
// - has violation -> report
// - no violation -> cleanup policy violations
func (pc *PolicyController) cleanupAndReport(engineResponses []response.EngineResponse) {
	logger := pc.log
	// generate Events
	eventInfos := generateEvents(pc.log, engineResponses)
	pc.eventGen.Add(eventInfos...)
	// create policy violation
	pvInfos := policyreport.GeneratePVsFromEngineResponse(engineResponses, logger)
	for i := range pvInfos {
		pvInfos[i].FromSync = true
	}

	pc.prGenerator.Add(pvInfos...)
	// cleanup existing violations if any
	// if there is any error in clean up, we dont re-queue the resource
	// it will be re-tried in the next controller cache resync
	pc.cleanUp(engineResponses)
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
