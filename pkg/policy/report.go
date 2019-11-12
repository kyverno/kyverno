package policy

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/event"
	clusterpv "github.com/nirmata/kyverno/pkg/clusterpolicyviolation"
)

// for each policy-resource response
// - has violation -> report
// - no violation -> cleanup policy violations(resource or resource owner)
func (pc *PolicyController) cleanupAndReport(engineResponses []engine.EngineResponse) {
	for _, eResponse := range engineResponses {
		if !eResponse.IsSuccesful() {
			// failure - policy/rule failed to apply on the resource
			reportEvents(eResponse, pc.eventGen)
			// generate policy violation
			// Only created on resource, not resource owners
			clusterpv.CreateClusterPV(pc.pvLister, pc.kyvernoClient, engineResponses)
		} else {
			// cleanup existing violations if any
			// if there is any error in clean up, we dont re-queue the resource
			// it will be re-tried in the next controller cache resync
			pc.cleanUpPolicyViolation(eResponse.PolicyResponse)
		}
	}
}

//reportEvents generates events for the failed resources
func reportEvents(engineResponse engine.EngineResponse, eventGen event.Interface) {
	if engineResponse.IsSuccesful() {
		return
	}
	glog.V(4).Infof("reporting results for policy '%s' application on resource '%s/%s/%s'", engineResponse.PolicyResponse.Policy, engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Namespace, engineResponse.PolicyResponse.Resource.Name)
	for _, rule := range engineResponse.PolicyResponse.Rules {
		if rule.Success {
			return
		}

		// generate event on resource for each failed rule
		glog.V(4).Infof("generation event on resource '%s/%s/%s' for policy '%s'", engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Namespace, engineResponse.PolicyResponse.Resource.Name, engineResponse.PolicyResponse.Policy)
		e := event.Info{}
		e.Kind = engineResponse.PolicyResponse.Resource.Kind
		e.Namespace = engineResponse.PolicyResponse.Resource.Namespace
		e.Name = engineResponse.PolicyResponse.Resource.Name
		e.Reason = "Failure"
		e.Message = fmt.Sprintf("policy '%s' (%s) rule '%s' failed to apply. %v", engineResponse.PolicyResponse.Policy, rule.Type, rule.Name, rule.Message)
		eventGen.Add(e)

	}
	// generate a event on policy for all failed rules
	glog.V(4).Infof("generation event on policy '%s'", engineResponse.PolicyResponse.Policy)
	e := event.Info{}
	e.Kind = "ClusterPolicy"
	e.Namespace = ""
	e.Name = engineResponse.PolicyResponse.Policy
	e.Reason = "Failure"
	e.Message = fmt.Sprintf("failed to apply policy '%s' rules '%v' on resource '%s/%s/%s'", engineResponse.PolicyResponse.Policy, engineResponse.GetFailedRules(), engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Namespace, engineResponse.PolicyResponse.Resource.Name)
	eventGen.Add(e)

}
