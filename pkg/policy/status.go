package policy

import (
	"log"
	"reflect"
	"sort"
	"sync"
	"time"

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
}

func NewStatusSync(
	client *versioned.Clientset,
	stopCh <-chan struct{},
	pMetaStore *policystore.PolicyStore,
) *StatSync {
	return &StatSync{
		cache: &statusCache{
			mu:   sync.RWMutex{},
			data: make(map[string]v1.PolicyStatus),
		},
		stop:        stopCh,
		client:      client,
		policyStore: pMetaStore,
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
	s.cache.data = make(map[string]v1.PolicyStatus)
	s.cache.mu.Unlock()

	for policyName, status := range nameToStatus {
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

func (s *StatSync) UpdatePolicyStatusWithViolationCount(policyName string, violatedRules []v1.ViolatedRule) {
	s.cache.mu.Lock()
	status := s.cache.data[policyName]

	var ruleNameToViolations = make(map[string]int)
	for _, rule := range violatedRules {
		ruleNameToViolations[rule.Name]++
	}

	for i := range status.Rules {
		status.ViolationCount += ruleNameToViolations[status.Rules[i].Name]
		status.Rules[i].ViolationCount += ruleNameToViolations[status.Rules[i].Name]
	}

	s.cache.data[policyName] = status
	s.cache.mu.Unlock()
}

func (s *StatSync) UpdatePolicyStatusWithGeneratedResourceCount(generateRequest v1.GenerateRequest) {
	s.cache.mu.Lock()
	status := s.cache.data[generateRequest.Spec.Policy]

	status.ResourcesGeneratedCount += len(generateRequest.Status.GeneratedResources)

	s.cache.data[generateRequest.Spec.Policy] = status
	s.cache.mu.Unlock()
}

func (s *StatSync) UpdateStatusWithMutateStats(resp response.EngineResponse) {
	if reflect.DeepEqual(response.EngineResponse{}, resp) {
		return
	}

	s.cache.mu.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := s.cache.data[resp.PolicyResponse.Policy]
	if !exist {
		policy, _ := s.policyStore.Get(resp.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range resp.PolicyResponse.Rules {
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

	s.cache.data[resp.PolicyResponse.Policy] = policyStatus
	s.cache.mu.Unlock()
}

func (s *StatSync) UpdateStatusWithValidateStats(resp response.EngineResponse) {
	if reflect.DeepEqual(response.EngineResponse{}, resp) {
		return
	}

	s.cache.mu.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := s.cache.data[resp.PolicyResponse.Policy]
	if !exist {
		policy, _ := s.policyStore.Get(resp.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range resp.PolicyResponse.Rules {
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
			if resp.PolicyResponse.ValidationFailureAction == "enforce" {
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

	s.cache.data[resp.PolicyResponse.Policy] = policyStatus
	s.cache.mu.Unlock()
}

func (s *StatSync) UpdateStatusWithGenerateStats(resp response.EngineResponse) {
	if reflect.DeepEqual(response.EngineResponse{}, resp) {
		return
	}

	s.cache.mu.Lock()
	var policyStatus v1.PolicyStatus
	policyStatus, exist := s.cache.data[resp.PolicyResponse.Policy]
	if !exist {
		policy, _ := s.policyStore.Get(resp.PolicyResponse.Policy)
		if policy != nil {
			policyStatus = policy.Status
		}
	}

	var nameToRule = make(map[string]v1.RuleStats, 0)
	for _, rule := range policyStatus.Rules {
		nameToRule[rule.Name] = rule
	}

	for _, rule := range resp.PolicyResponse.Rules {
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

	s.cache.data[resp.PolicyResponse.Policy] = policyStatus
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
