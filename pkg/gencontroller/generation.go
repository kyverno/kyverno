package gencontroller

import (
	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
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

func (c *Controller) processPolicy(ns *corev1.Namespace, p *v1alpha1.Policy) error {
	policyInfo := info.NewPolicyInfo(p.Name,
		ns.Kind,
		ns.Name,
		"") // Namespace has no namespace..WOW

	ruleInfos := engine.GenerateNew(c.client, p, ns)
	policyInfo.AddRuleInfos(ruleInfos)
	if !policyInfo.IsSuccessful() {
		glog.Infof("Failed to apply policy %s on resource %s %s", p.Name, ns.Kind, ns.Name)
		for _, r := range ruleInfos {
			glog.Warning(r.Msgs)
		}
	} else {
		glog.Infof("Generation from policy %s has succesfully applied to %s %s", p.Name, ns.Kind, ns.Name)
	}

	//TODO Generate policy Violations and corresponding events based on policyInfo
	return nil
}
