package policy

import (
	"fmt"

	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/policyviolation"
)

func (pc *PolicyController) report(policyInfos []info.PolicyInfo) {
	// generate events
	// generate policy violations
	for _, policyInfo := range policyInfos {
		// events
		// success - policy applied on resource
		// failure - policy/rule failed to apply on the resource
		reportPolicy(policyInfo, pc.eventGen)
		// policy violations
		// failure - policy/rule failed to apply on the resource
	}

	// generate policy violation
	policyviolation.GeneratePolicyViolations(pc.pvListerSynced, pc.pvLister, pc.kyvernoClient, policyInfos)

}

func reportPolicy(policyInfo info.PolicyInfo, eventGen event.Interface) {

	if policyInfo.IsSuccessful() {
		return
	}

	for _, rule := range policyInfo.Rules {
		if rule.IsSuccessful() {
			continue
		}

		// generate event on resource for each failed rule
		e := &event.Info{}
		e.Kind = policyInfo.RKind
		e.Namespace = policyInfo.RNamespace
		e.Name = policyInfo.RName
		e.Reason = "Failure"
		e.Message = fmt.Sprintf("policy %s (%s) rule %s failed to apply. %v", policyInfo.Name, rule.RuleType.String(), rule.Name, rule.GetErrorString())
		eventGen.Add(e)

		// generate policy violation for each failed rule

	}
	// generate a event on policy for all failed rules
	e := &event.Info{}
	e.Kind = "Policy"
	e.Namespace = ""
	e.Name = policyInfo.Name
	e.Reason = "Failure"
	e.Message = fmt.Sprintf("failed to apply rules %s on resource %s/%s/%s", policyInfo.FailedRules(), policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName)
	eventGen.Add(e)
}
