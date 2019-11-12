package namespace

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/event"
	policyviolation "github.com/nirmata/kyverno/pkg/policyviolation"
)

func (nsc *NamespaceController) report(engineResponses []engine.EngineResponse) {
	// generate events
	eventInfos := generateEvents(engineResponses)
	nsc.eventGen.Add(eventInfos...)
	// generate policy violations
	pvInfos := generatePVs(engineResponses)
	nsc.pvGenerator.Add(pvInfos...)
}

func generatePVs(ers []engine.EngineResponse) []policyviolation.Info {
	var pvInfos []policyviolation.Info
	for _, er := range ers {
		// ignore creation of PV for resoruces that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			glog.V(4).Infof("resource %v, has not been assigned a name, not creating a policy violation for it", er.PolicyResponse.Resource)
			continue
		}
		if er.IsSuccesful() {
			continue
		}
		glog.V(4).Infof("Building policy violation for engine response %v", er)
		// build policy violation info
		pvInfos = append(pvInfos, buildPVInfo(er))
	}

	return pvInfos
}

func buildPVInfo(er engine.EngineResponse) policyviolation.Info {
	info := policyviolation.Info{
		Blocked:    false,
		PolicyName: er.PolicyResponse.Policy,
		Resource:   er.PatchedResource,
		Rules:      buildViolatedRules(er),
	}
	return info
}

func buildViolatedRules(er engine.EngineResponse) []kyverno.ViolatedRule {
	var violatedRules []kyverno.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		vrule := kyverno.ViolatedRule{
			Name:    rule.Name,
			Type:    rule.Type,
			Message: rule.Message,
		}
		violatedRules = append(violatedRules, vrule)
	}
	return violatedRules
}

func generateEvents(ers []engine.EngineResponse) []event.Info {
	var eventInfos []event.Info
	for _, er := range ers {
		if er.IsSuccesful() {
			continue
		}
		eventInfos = append(eventInfos, generateEventsPerEr(er)...)
	}
	return eventInfos
}

func generateEventsPerEr(er engine.EngineResponse) []event.Info {
	var eventInfos []event.Info
	glog.V(4).Infof("reporting results for policy '%s' application on resource '%s/%s/%s'", er.PolicyResponse.Policy, er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		// generate event on resource for each failed rule
		glog.V(4).Infof("generation event on resource '%s/%s' for policy '%s'", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Name, er.PolicyResponse.Policy)
		e := event.Info{}
		e.Kind = er.PolicyResponse.Resource.Kind
		e.Namespace = "" // event generate on namespace resource
		e.Name = er.PolicyResponse.Resource.Name
		e.Reason = "Failure"
		e.Message = fmt.Sprintf("policy '%s' (%s) rule '%s' failed to apply. %v", er.PolicyResponse.Policy, rule.Type, rule.Name, rule.Message)
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
	e.Reason = "Failure"
	e.Message = fmt.Sprintf("failed to apply policy '%s' rules '%v' on resource '%s/%s/%s'", er.PolicyResponse.Policy, er.GetFailedRules(), er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
	return eventInfos
}
