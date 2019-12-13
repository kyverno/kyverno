package policy

import (
	"sync"
	"time"

	"github.com/golang/glog"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

//PolicyStatusAggregator stores information abt aggregation
type PolicyStatusAggregator struct {
	// time since we start aggregating the stats
	startTime time.Time
	// channel to recieve stats
	ch chan PolicyStat
	//TODO: lock based on key, possibly sync.Map ?
	//sync RW for policyData
	mux sync.RWMutex
	// stores aggregated stats for policy
	policyData map[string]PolicyStatInfo
}

//NewPolicyStatAggregator returns a new policy status
func NewPolicyStatAggregator(client *kyvernoclient.Clientset) *PolicyStatusAggregator {
	psa := PolicyStatusAggregator{
		startTime:  time.Now(),
		ch:         make(chan PolicyStat),
		policyData: map[string]PolicyStatInfo{},
	}
	return &psa
}

//Run begins aggregator
func (psa *PolicyStatusAggregator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.V(4).Info("Started aggregator for policy status stats")
	defer func() {
		glog.V(4).Info("Shutting down aggregator for policy status stats")
	}()
	for i := 0; i < workers; i++ {
		go wait.Until(psa.process, time.Second, stopCh)
	}
	<-stopCh
}

func (psa *PolicyStatusAggregator) process() {
	// As mutation and validation are handled seperately
	// ideally we need to combine the exection time from both for a policy
	// but its tricky to detect here the type of rules policy contains
	// so we dont combine the results, but instead compute the execution time for
	// mutation & validation rules seperately
	for r := range psa.ch {
		glog.V(4).Infof("recieved policy stats %v", r)
		psa.aggregate(r)
	}
}

func (psa *PolicyStatusAggregator) aggregate(ps PolicyStat) {
	func() {
		glog.V(4).Infof("write lock update policy %s", ps.PolicyName)
		psa.mux.Lock()
	}()
	defer func() {
		glog.V(4).Infof("write Unlock update policy %s", ps.PolicyName)
		psa.mux.Unlock()
	}()

	if len(ps.Stats.Rules) == 0 {
		glog.V(4).Infof("ignoring stats, as no rule was applied")
		return
	}

	info, ok := psa.policyData[ps.PolicyName]
	if !ok {
		psa.policyData[ps.PolicyName] = ps.Stats
		glog.V(4).Infof("added stats for policy %s", ps.PolicyName)
		return
	}
	// aggregate policy information
	info.RulesAppliedCount = info.RulesAppliedCount + ps.Stats.RulesAppliedCount
	if ps.Stats.ResourceBlocked == 1 {
		info.ResourceBlocked++
	}
	var zeroDuration time.Duration
	if info.MutationExecutionTime != zeroDuration {
		info.MutationExecutionTime = (info.MutationExecutionTime + ps.Stats.MutationExecutionTime) / 2
		glog.V(4).Infof("updated avg mutation time %v", info.MutationExecutionTime)
	} else {
		info.MutationExecutionTime = ps.Stats.MutationExecutionTime
	}
	if info.ValidationExecutionTime != zeroDuration {
		info.ValidationExecutionTime = (info.ValidationExecutionTime + ps.Stats.ValidationExecutionTime) / 2
		glog.V(4).Infof("updated avg validation time %v", info.ValidationExecutionTime)
	} else {
		info.ValidationExecutionTime = ps.Stats.ValidationExecutionTime
	}
	if info.GenerationExecutionTime != zeroDuration {
		info.GenerationExecutionTime = (info.GenerationExecutionTime + ps.Stats.GenerationExecutionTime) / 2
		glog.V(4).Infof("updated avg generation time %v", info.GenerationExecutionTime)
	} else {
		info.GenerationExecutionTime = ps.Stats.GenerationExecutionTime
	}
	// aggregate rule details
	info.Rules = aggregateRules(info.Rules, ps.Stats.Rules)
	// update
	psa.policyData[ps.PolicyName] = info
	glog.V(4).Infof("updated stats for policy %s", ps.PolicyName)
}

func aggregateRules(old []RuleStatinfo, update []RuleStatinfo) []RuleStatinfo {
	var zeroDuration time.Duration
	searchRule := func(list []RuleStatinfo, key string) *RuleStatinfo {
		for _, v := range list {
			if v.RuleName == key {
				return &v
			}
		}
		return nil
	}
	newRules := []RuleStatinfo{}
	// search for new rules in old rules and update it
	for _, updateR := range update {
		if updateR.ExecutionTime != zeroDuration {
			if rule := searchRule(old, updateR.RuleName); rule != nil {
				rule.ExecutionTime = (rule.ExecutionTime + updateR.ExecutionTime) / 2
				rule.RuleAppliedCount = rule.RuleAppliedCount + updateR.RuleAppliedCount
				rule.RulesFailedCount = rule.RulesFailedCount + updateR.RulesFailedCount
				rule.MutationCount = rule.MutationCount + updateR.MutationCount
				newRules = append(newRules, *rule)
			} else {
				newRules = append(newRules, updateR)
			}
		}
	}
	return newRules
}

//GetPolicyStats returns the policy stats
func (psa *PolicyStatusAggregator) GetPolicyStats(policyName string) PolicyStatInfo {
	func() {
		glog.V(4).Infof("read lock update policy %s", policyName)
		psa.mux.RLock()
	}()
	defer func() {
		glog.V(4).Infof("read Unlock update policy %s", policyName)
		psa.mux.RUnlock()
	}()
	glog.V(4).Infof("read stats for policy %s", policyName)
	return psa.policyData[policyName]
}

//RemovePolicyStats rmves policy stats records
func (psa *PolicyStatusAggregator) RemovePolicyStats(policyName string) {
	func() {
		glog.V(4).Infof("write lock update policy %s", policyName)
		psa.mux.Lock()
	}()
	defer func() {
		glog.V(4).Infof("write Unlock update policy %s", policyName)
		psa.mux.Unlock()
	}()
	glog.V(4).Infof("removing stats for policy %s", policyName)
	delete(psa.policyData, policyName)
}

//PolicyStatusInterface provides methods to modify policyStatus
type PolicyStatusInterface interface {
	SendStat(stat PolicyStat)
	// UpdateViolationCount(policyName string, pvList []*kyverno.PolicyViolation) error
}

//PolicyStat stored stats for policy
type PolicyStat struct {
	PolicyName string
	Stats      PolicyStatInfo
}

type PolicyStatInfo struct {
	MutationExecutionTime   time.Duration
	ValidationExecutionTime time.Duration
	GenerationExecutionTime time.Duration
	RulesAppliedCount       int
	ResourceBlocked         int
	Rules                   []RuleStatinfo
}

type RuleStatinfo struct {
	RuleName         string
	ExecutionTime    time.Duration
	RuleAppliedCount int
	RulesFailedCount int
	MutationCount    int
}

//SendStat sends the stat information for aggregation
func (psa *PolicyStatusAggregator) SendStat(stat PolicyStat) {
	glog.V(4).Infof("sending policy stats: %v", stat)
	// Send over channel
	psa.ch <- stat
}

//GetPolicyStatusAggregator returns interface to send policy status stats
func (pc *PolicyController) GetPolicyStatusAggregator() PolicyStatusInterface {
	return pc.statusAggregator
}
