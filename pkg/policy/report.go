package policy

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyviolation"
)

// for each policy-resource response
// - has violation -> report
// - no violation -> cleanup policy violations(resource or resource owner)
func (pc *PolicyController) cleanupAndReport(engineResponses []engine.EngineResponse) {
	// generate Events
	eventInfos := generateEvents(engineResponses)
	pc.eventGen.Add(eventInfos...)
	// create policy violation
	pvInfos := generatePVs(engineResponses)
	pc.pvGenerator.Add(pvInfos...)
	// cleanup existing violations if any
	// if there is any error in clean up, we dont re-queue the resource
	// it will be re-tried in the next controller cache resync
	pc.cleanUp(engineResponses)
}

func (pc *PolicyController) cleanUp(ers []engine.EngineResponse) {
	for _, er := range ers {
		if !er.IsSuccesful() {
			continue
		}
		if len(er.PolicyResponse.Rules) == 0 {
			continue
		}
		// clean up after the policy has been corrected
		pc.cleanUpPolicyViolation(er.PolicyResponse)
	}
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
