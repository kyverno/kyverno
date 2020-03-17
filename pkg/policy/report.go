package policy

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyviolation"
)

// for each policy-resource response
// - has violation -> report
// - no violation -> cleanup policy violations
func (pc *PolicyController) cleanupAndReport(engineResponses []response.EngineResponse) {
	logger := pc.log
	// generate Events
	eventInfos := generateEvents(engineResponses)
	pc.eventGen.Add(eventInfos...)
	// create policy violation
	pvInfos := policyviolation.GeneratePVsFromEngineResponse(engineResponses, logger)
	pc.pvGenerator.Add(pvInfos...)
	// cleanup existing violations if any
	// if there is any error in clean up, we dont re-queue the resource
	// it will be re-tried in the next controller cache resync
	pc.cleanUp(engineResponses)
}

func generateEvents(ers []response.EngineResponse) []event.Info {
	var eventInfos []event.Info
	for _, er := range ers {
		if er.IsSuccesful() {
			continue
		}
		eventInfos = append(eventInfos, generateEventsPerEr(er)...)
	}
	return eventInfos
}

func generateEventsPerEr(er response.EngineResponse) []event.Info {
	var eventInfos []event.Info
	glog.V(4).Infof("reporting results for policy '%s' application on resource '%s/%s/%s'", er.PolicyResponse.Policy, er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		// generate event on resource for each failed rule
		glog.V(4).Infof("generation event on resource '%s/%s/%s' for policy '%s'", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name, er.PolicyResponse.Policy)
		e := event.Info{}
		e.Kind = er.PolicyResponse.Resource.Kind
		e.Namespace = er.PolicyResponse.Resource.Namespace
		e.Name = er.PolicyResponse.Resource.Name
		e.Reason = event.PolicyViolation.String()
		e.Source = event.PolicyController
		e.Message = fmt.Sprintf("policy '%s' (%s) rule '%s' not satisfied. %v", er.PolicyResponse.Policy, rule.Type, rule.Name, rule.Message)
		eventInfos = append(eventInfos, e)
	}
	if er.IsSuccesful() {
		return eventInfos
	}

	// generate a event on policy for all failed rules
	glog.V(4).Infof("generation event on policy '%s'", er.PolicyResponse.Policy)
	e := event.Info{}
	e.Kind = "ClusterPolicy"
	e.Namespace = ""
	e.Name = er.PolicyResponse.Policy
	e.Reason = event.PolicyViolation.String()
	e.Source = event.PolicyController
	e.Message = fmt.Sprintf("policy '%s' rules '%v' not satisfied on resource '%s/%s/%s'", er.PolicyResponse.Policy, er.GetFailedRules(), er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
	eventInfos = append(eventInfos, e)
	return eventInfos
}
