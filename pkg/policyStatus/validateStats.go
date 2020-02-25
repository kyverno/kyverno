package policyStatus

import (
	"reflect"
	"sort"
	"time"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

type validateStats struct {
	s    *Sync
	resp response.EngineResponse
}

func (s *Sync) UpdateStatusWithValidateStats(resp response.EngineResponse) {
	s.listener <- &validateStats{
		s:    s,
		resp: resp,
	}
}

func (vs *validateStats) updateStatus() {
	if reflect.DeepEqual(response.EngineResponse{}, vs.resp) {
		return
	}

	vs.s.cache.mutex.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := vs.s.cache.data[vs.resp.PolicyResponse.Policy]
	if !exist {
		policy, _ := vs.s.policyStore.Get(vs.resp.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range vs.resp.PolicyResponse.Rules {
		ruleStat := nameToRule[rule.Name]
		ruleStat.Name = rule.Name

		averageOver := int64(ruleStat.AppliedCount + ruleStat.FailedCount)
		ruleStat.ExecutionTime = updateAverageTime(
			rule.ProcessingTime,
			ruleStat.ExecutionTime,
			averageOver).String()

		if rule.Success {
			policyStatus.RulesAppliedCount++
			ruleStat.AppliedCount++
		} else {
			policyStatus.RulesFailedCount++
			ruleStat.FailedCount++
			if vs.resp.PolicyResponse.ValidationFailureAction == "enforce" {
				policyStatus.ResourcesBlockedCount++
				ruleStat.ResourcesBlockedCount++
			}
		}

		nameToRule[rule.Name] = ruleStat
	}

	var policyAverageExecutionTime time.Duration
	var ruleStats = make([]v1.RuleStats, 0, len(nameToRule))
	for _, ruleStat := range nameToRule {
		executionTime, err := time.ParseDuration(ruleStat.ExecutionTime)
		if err == nil {
			policyAverageExecutionTime += executionTime
		}
		ruleStats = append(ruleStats, ruleStat)
	}

	sort.Slice(ruleStats, func(i, j int) bool {
		return ruleStats[i].Name < ruleStats[j].Name
	})

	policyStatus.AvgExecutionTime = policyAverageExecutionTime.String()
	policyStatus.Rules = ruleStats

	vs.s.cache.data[vs.resp.PolicyResponse.Policy] = policyStatus
	vs.s.cache.mutex.Unlock()
}
