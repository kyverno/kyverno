package policyStatus

import (
	"reflect"
	"sort"
	"time"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

type generateStats struct {
	s    *Sync
	resp response.EngineResponse
}

func (s *Sync) UpdateStatusWithGenerateStats(resp response.EngineResponse) {
	s.listener <- &generateStats{
		s:    s,
		resp: resp,
	}
}

func (gs *generateStats) updateStatus() {
	if reflect.DeepEqual(response.EngineResponse{}, gs.resp) {
		return
	}

	gs.s.cache.mutex.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := gs.s.cache.data[gs.resp.PolicyResponse.Policy]
	if !exist {
		policy, _ := gs.s.policyStore.Get(gs.resp.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range gs.resp.PolicyResponse.Rules {
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

	gs.s.cache.data[gs.resp.PolicyResponse.Policy] = policyStatus
	gs.s.cache.mutex.Unlock()
}
