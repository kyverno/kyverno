package policy

import (
	"log"
	"sort"
	"sync"
	"time"

	v12 "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"

	"github.com/nirmata/kyverno/pkg/policystore"

	"github.com/nirmata/kyverno/pkg/engine/response"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nirmata/kyverno/pkg/client/clientset/versioned"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type statusCache struct {
	mu   sync.RWMutex
	data map[string]v1.PolicyStatus
}

type StatSync struct {
	cache       *statusCache
	stop        <-chan struct{}
	client      *versioned.Clientset
	policyStore *policystore.PolicyStore
	cpvLister   v12.ClusterPolicyViolationLister
	pvLister    v12.PolicyViolationLister
}

func NewStatusSync(
	client *versioned.Clientset,
	stopCh <-chan struct{},
	pMetaStore *policystore.PolicyStore,
	cpvLister v12.ClusterPolicyViolationLister,
	pvLister v12.PolicyViolationLister,
) *StatSync {
	return &StatSync{
		cache: &statusCache{
			mu:   sync.RWMutex{},
			data: make(map[string]v1.PolicyStatus),
		},
		stop:        stopCh,
		client:      client,
		policyStore: pMetaStore,
		cpvLister:   cpvLister,
		pvLister:    pvLister,
	}
}

func (s *StatSync) Run() {
	// update policy status every 10 seconds - waits for previous updateStatus to complete
	wait.Until(s.updateStats, 1*time.Second, s.stop)
	<-s.stop
	s.updateStats()
}

func (s *StatSync) updateStats() {
	s.cache.mu.Lock()
	var nameToStatus = make(map[string]v1.PolicyStatus, len(s.cache.data))
	for k, v := range s.cache.data {
		nameToStatus[k] = v
	}
	s.cache.mu.Unlock()

	for policyName, status := range nameToStatus {
		cpvList, _ := s.getClusterPolicyViolationForPolicy(policyName)
		pvList, _ := s.getNamespacedPolicyViolationForPolicy(policyName)
		updateStatusWithViolationCount(&status, cpvList, pvList)

		var policy = &v1.ClusterPolicy{}
		policy, err := s.policyStore.Get(policyName)
		if err != nil {
			continue
		}
		policy.Status = status
		_, err = s.client.KyvernoV1().ClusterPolicies().UpdateStatus(policy)
		if err != nil {
			log.Println(err)
		}
	}
}

func (s *StatSync) UpdateStatusWithMutateStats(response response.EngineResponse) {
	s.cache.mu.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := s.cache.data[response.PolicyResponse.Policy]
	if !exist {
		policy, _ := s.policyStore.Get(response.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range response.PolicyResponse.Rules {
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

	s.cache.data[response.PolicyResponse.Policy] = policyStatus
	s.cache.mu.Unlock()
}

func (s *StatSync) UpdateStatusWithValidateStats(response response.EngineResponse) {
	s.cache.mu.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := s.cache.data[response.PolicyResponse.Policy]
	if !exist {
		policy, _ := s.policyStore.Get(response.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range response.PolicyResponse.Rules {
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
			if response.PolicyResponse.ValidationFailureAction == "enforce" {
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

	s.cache.data[response.PolicyResponse.Policy] = policyStatus
	s.cache.mu.Unlock()
}

func (s *StatSync) UpdateStatusWithGenerateStats(response response.EngineResponse) {
	s.cache.mu.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := s.cache.data[response.PolicyResponse.Policy]
	if !exist {
		policy, _ := s.policyStore.Get(response.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range response.PolicyResponse.Rules {
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

	s.cache.data[response.PolicyResponse.Policy] = policyStatus
	s.cache.mu.Unlock()
}

func updateAverageTime(newTime time.Duration, oldAverageTimeString string, averageOver int64) time.Duration {
	if averageOver == 0 {
		return newTime
	}
	oldAverageExecutionTime, _ := time.ParseDuration(oldAverageTimeString)
	numerator := (oldAverageExecutionTime.Nanoseconds() * averageOver) + newTime.Nanoseconds()
	denominator := averageOver + 1
	newAverageTimeInNanoSeconds := numerator / denominator
	return time.Duration(newAverageTimeInNanoSeconds) * time.Nanosecond
}

func (s *StatSync) getClusterPolicyViolationForPolicy(policy string) ([]*v1.ClusterPolicyViolation, error) {
	policySelector, err := buildPolicyLabel(policy)
	if err != nil {
		return nil, err
	}
	// Get List of cluster policy violation
	cpvList, err := s.cpvLister.List(policySelector)
	if err != nil {
		return nil, err
	}
	return cpvList, nil
}

func (s *StatSync) getNamespacedPolicyViolationForPolicy(policy string) ([]*v1.PolicyViolation, error) {
	policySelector, err := buildPolicyLabel(policy)
	if err != nil {
		return nil, err
	}
	// Get List of cluster policy violation
	nspvList, err := s.pvLister.List(policySelector)
	if err != nil {
		return nil, err
	}
	return nspvList, nil

}

func updateStatusWithViolationCount(status *v1.PolicyStatus, cpvList []*v1.ClusterPolicyViolation, pvList []*v1.PolicyViolation) {

	status.ViolationCount = len(cpvList) + len(pvList)

	var ruleNameToNumberOfViolations = make(map[string]int)

	for _, cpv := range cpvList {
		for _, violatedRule := range cpv.Spec.ViolatedRules {
			ruleNameToNumberOfViolations[violatedRule.Name]++
		}
	}

	for _, pv := range pvList {
		for _, violatedRule := range pv.Spec.ViolatedRules {
			ruleNameToNumberOfViolations[violatedRule.Name]++
		}
	}

	for i, rule := range status.Rules {
		status.Rules[i].ViolationCount = ruleNameToNumberOfViolations[rule.Name]
	}
}
