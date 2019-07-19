package gencontroller

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/info"
	violation "github.com/nirmata/kyverno/pkg/violation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Controller) processNamespace(ns *corev1.Namespace) error {
	//Get all policies and then verify if the namespace matches any of the defined selectors
	policies, err := c.listPolicies(ns)
	if err != nil {
		return err
	}
	// process policy on namespace
	for _, p := range policies {
		c.processPolicy(ns, p)
	}

	return nil
}

func (c *Controller) listPolicies(ns *corev1.Namespace) ([]*v1alpha1.Policy, error) {
	var fpolicies []*v1alpha1.Policy
	policies, err := c.policyLister.List(labels.NewSelector())
	if err != nil {
		glog.Error("Unable to connect to policy controller. Unable to access policies not applying GENERATION rules")
		return nil, err
	}
	for _, p := range policies {
		// Check if the policy contains a generatoin rule
		for _, r := range p.Spec.Rules {
			if r.Generation != nil {
				// Check if the resource meets the description
				if namespaceMeetsRuleDescription(ns, r.ResourceDescription) {
					fpolicies = append(fpolicies, p)
					break
				}
			}
		}
	}

	return fpolicies, nil
}

func (c *Controller) processPolicy(ns *corev1.Namespace, p *v1alpha1.Policy) {
	var eventInfo *event.Info
	var onViolation bool
	var msg string

	policyInfo := info.NewPolicyInfo(p.Name,
		"Namespace",
		ns.Name,
		"",
		p.Spec.ValidationFailureAction) // Namespace has no namespace..WOW

	ruleInfos := engine.GenerateNew(c.client, p, ns)
	policyInfo.AddRuleInfos(ruleInfos)

	if !policyInfo.IsSuccessful() {
		glog.Infof("Failed to apply policy %s on resource %s %s", p.Name, ns.Kind, ns.Name)
		for _, r := range ruleInfos {
			glog.Warning(r.Msgs)

			if msg = strings.Join(r.Msgs, " "); strings.Contains(msg, "rule configuration not present in resource") {
				onViolation = true
				msg = fmt.Sprintf(`Resource creation violates generate rule '%s' of policy '%s'`, r.Name, policyInfo.Name)
			}
		}

		if onViolation {
			glog.Infof("Adding violation for generation rule of policy %s\n", policyInfo.Name)
			v := violation.BuldNewViolation(policyInfo.Name, policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, event.PolicyViolation.String(), policyInfo.GetFailedRules())
			c.violationBuilder.Add(v)
		} else {
			eventInfo = event.NewEvent(policyKind, "", policyInfo.Name, event.RequestBlocked,
				event.FPolicyApplyBlockCreate, policyInfo.RName, policyInfo.GetRuleNames(false))

			glog.V(2).Infof("Request blocked event info has prepared for %s/%s\n", policyKind, policyInfo.Name)

			c.eventController.Add(eventInfo)
		}
		return
	}

	glog.Infof("Generation from policy %s has succesfully applied to %s/%s", p.Name, policyInfo.RKind, policyInfo.RName)

	eventInfo = event.NewEvent(policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName,
		event.PolicyApplied, event.SRulesApply, policyInfo.GetRuleNames(true), policyInfo.Name)

	glog.V(2).Infof("Success event info has prepared for %s/%s\n", policyInfo.RKind, policyInfo.RName)

	c.eventController.Add(eventInfo)
}
