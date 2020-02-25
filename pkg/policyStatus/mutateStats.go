package policyStatus

import (
	"reflect"
	"sort"
	"time"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

type mutateStats struct {
	s    *Sync
	resp response.EngineResponse
}

func (s *Sync) UpdateStatusWithMutateStats(resp response.EngineResponse) {
	s.listener <- &mutateStats{
		s:    s,
		resp: resp,
	}

}

func (ms *mutateStats) updateStatus() {
	if reflect.DeepEqual(response.EngineResponse{}, ms.resp) {
		return
	}

	ms.s.cache.mutex.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := ms.s.cache.data[ms.resp.PolicyResponse.Policy]
	if !exist {
		policy, _ := ms.s.policyStore.Get(ms.resp.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range ms.resp.PolicyResponse.Rules {
		ruleStat := nameToRule[rule.Name]
		ruleStat.Name = rule.Name

		averageOver := int64(ruleStat.AppliedCount + ruleStat.FailedCount)
		ruleStat.ExecutionTime = updateAverageTime(
			rule.ProcessingTime,
			ruleStat.ExecutionTime,
			averageOver).String()

		if rule.Success {
			policyStatus.RulesAppliedCount++
			policyStatus.ResourcesMutatedCount++
			ruleStat.AppliedCount++
			ruleStat.ResourcesMutatedCount++
		} else {
			policyStatus.RulesFailedCount++
			ruleStat.FailedCount++
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

	ms.s.cache.data[ms.resp.PolicyResponse.Policy] = policyStatus
	ms.s.cache.mutex.Unlock()
}
