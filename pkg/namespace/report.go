package namespace

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyviolation"
)

func (nsc *NamespaceController) report(engineResponses []engine.EngineResponseNew) {
	// generate events
	// generate policy violations
	for _, er := range engineResponses {
		// events
		// success - policy applied on resource
		// failure - policy/rule failed to apply on the resource
		reportEvents(er, nsc.eventGen)
		// policy violations
		// failure - policy/rule failed to apply on the resource
	}
	// generate policy violation
	policyviolation.CreatePV(nsc.pvLister, nsc.kyvernoClient, engineResponses)
}

//reportEvents generates events for the failed resources
func reportEvents(engineResponse engine.EngineResponseNew, eventGen event.Interface) {
	if engineResponse.IsSuccesful() {
		return
	}
	glog.V(4).Infof("reporting results for policy '%s' application on resource '%s/%s/%s'", engineResponse.PolicyResponse.Policy, engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Namespace, engineResponse.PolicyResponse.Resource.Name)
	for _, rule := range engineResponse.PolicyResponse.Rules {
		if rule.Success {
			return
		}
		// generate event on resource for each failed rule
		glog.V(4).Infof("generation event on resource '%s/%s' for policy '%s'", engineResponse.PolicyResponse.Resource.Kind, engineResponse.PolicyResponse.Resource.Name, engineResponse.PolicyResponse.Policy)
		e := event.Info{}
		e.Kind = engineResponse.PolicyResponse.Resource.Kind
		e.Namespace = "" // event generate on namespace resource
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
