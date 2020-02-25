package policyStatus

import v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

type violationCount struct {
	sync          *Sync
	policyName    string
	violatedRules []v1.ViolatedRule
}

func (s *Sync) UpdatePolicyStatusWithViolationCount(policyName string, violatedRules []v1.ViolatedRule) {
	s.listener <- &violationCount{
		sync:          s,
		policyName:    policyName,
		violatedRules: violatedRules,
	}
}

func (vc *violationCount) updateStatus() {
	vc.sync.cache.mutex.Lock()
	status, exist := vc.sync.cache.data[vc.policyName]
	if !exist {
		policy, _ := vc.sync.policyStore.Get(vc.policyName)
		if policy != nil {
			status = policy.Status
		}
	}

	var ruleNameToViolations = make(map[string]int)
	for _, rule := range vc.violatedRules {
		ruleNameToViolations[rule.Name]++
	}

	for i := range status.Rules {
		status.ViolationCount += ruleNameToViolations[status.Rules[i].Name]
		status.Rules[i].ViolationCount += ruleNameToViolations[status.Rules[i].Name]
	}

	vc.sync.cache.data[vc.policyName] = status
	vc.sync.cache.mutex.Unlock()
}
