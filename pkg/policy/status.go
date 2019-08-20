package policy

import (
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

type PolicyStatus struct {
	// average time required to process the policy rules on a resource
	avgExecutionTime time.Duration
	// Count of rules that were applied succesfully
	rulesAppliedCount int
	// Count of resources for whom update/create api requests were blocked as the resoruce did not satisfy the policy rules
	resourcesBlockedCount int
	// Count of the resource for whom the mutation rules were applied succesfully
	resourcesMutatedCount int
}

type PolicyStatusAggregator struct {
	// time since we start aggregating the stats
	startTime time.Time
	// channel to recieve stats
	ch chan PolicyStat
	// update polict status
	psControl PStatusControlInterface
	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.PolicyLister
	// UpdateViolationCount and SendStat can update same policy status
	// we need to sync the updates using policyName
	policyUpdateData sync.Map
}

//NewPolicyStatAggregator returns a new policy status
func NewPolicyStatAggregator(client *kyvernoclient.Clientset, pLister kyvernolister.PolicyLister) *PolicyStatusAggregator {
	psa := PolicyStatusAggregator{
		startTime: time.Now(),
		ch:        make(chan PolicyStat),
	}
	psa.pLister = pLister
	psa.psControl = PSControl{Client: client}
	//TODO: add WaitGroup
	return &psa
}

func (psa *PolicyStatusAggregator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.V(4).Info("Started aggregator for policy status stats")
	defer func() {
		glog.V(4).Info("Shutting down aggregator for policy status stats")
	}()
	for i := 0; i < workers; i++ {
		go wait.Until(psa.aggregate, time.Second, stopCh)
	}
}
func (psa *PolicyStatusAggregator) aggregate() {
	// As mutation and validation are handled seperately
	// ideally we need to combine the exection time from both for a policy
	// but its tricky to detect here the type of rules policy contains
	// so we dont combine the results, but instead compute the execution time for
	// mutation & validation rules seperately
	for r := range psa.ch {
		glog.V(4).Infof("recieved policy stats %v", r)
		if err := psa.updateStats(r); err != nil {
			glog.Info("Failed to update stats for policy %s: %v", r.PolicyName, err)
		}
	}

}

func (psa *PolicyStatusAggregator) updateStats(stats PolicyStat) error {
	func() {
		glog.V(4).Infof("lock updates for policy name %s", stats.PolicyName)
		// Lock the update for policy
		psa.policyUpdateData.Store(stats.PolicyName, struct{}{})
	}()
	defer func() {
		glog.V(4).Infof("Unlock updates for policy name %s", stats.PolicyName)
		psa.policyUpdateData.Delete(stats.PolicyName)
	}()
	// get policy
	policy, err := psa.pLister.Get(stats.PolicyName)
	if err != nil {
		glog.V(4).Infof("failed to get policy %s. Unable to update violation count: %v", stats.PolicyName, err)
		return err
	}
	glog.V(4).Infof("updating stats for policy %s", policy.Name)
	// update the statistics
	// policy.Status

	return nil
}

type PolicyStatusInterface interface {
	SendStat(stat PolicyStat)
	UpdateViolationCount(policyName string, pvList []*kyverno.PolicyViolation) error
}

type PolicyStat struct {
	PolicyName              string
	MutationExecutionTime   time.Duration
	ValidationExecutionTime time.Duration
	RulesAppliedCount       int
	ResourceBlocked         bool
}

//SendStat sends the stat information for aggregation
func (psa *PolicyStatusAggregator) SendStat(stat PolicyStat) {
	glog.V(4).Infof("sending policy stats: %v", stat)
	// Send over channel
	psa.ch <- stat
}

//UpdateViolationCount updates the active violation count
func (psa *PolicyStatusAggregator) UpdateViolationCount(policyName string, pvList []*kyverno.PolicyViolation) error {
	func() {
		glog.V(4).Infof("lock updates for policy name %s", policyName)
		// Lock the update for policy
		psa.policyUpdateData.Store(policyName, struct{}{})
	}()
	defer func() {
		glog.V(4).Infof("Unlock updates for policy name %s", policyName)
		psa.policyUpdateData.Delete(policyName)
	}()
	// get policy
	policy, err := psa.pLister.Get(policyName)
	if err != nil {
		glog.V(4).Infof("failed to get policy %s. Unable to update violation count: %v", policyName, err)
		return err
	}

	newStatus := calculateStatus(pvList)
	if reflect.DeepEqual(newStatus, policy.Status) {
		// no update to status
		return nil
	}
	// update status
	newPolicy := policy
	newPolicy.Status = newStatus

	return psa.psControl.UpdatePolicyStatus(newPolicy)
}

func calculateStatus(pvList []*kyverno.PolicyViolation) kyverno.PolicyStatus {
	violationCount := len(pvList)
	status := kyverno.PolicyStatus{
		Violations: violationCount,
	}
	return status
}

//GetPolicyStatusAggregator returns interface to send policy status stats
func (pc *PolicyController) GetPolicyStatusAggregator() PolicyStatusInterface {
	return pc.statusAggregator
}

//PStatusControlInterface Provides interface to operate on policy status
type PStatusControlInterface interface {
	UpdatePolicyStatus(newPolicy *kyverno.Policy) error
}

type PSControl struct {
	Client kyvernoclient.Interface
}

//UpdatePolicyStatus update policy status
func (c PSControl) UpdatePolicyStatus(newPolicy *kyverno.Policy) error {
	_, err := c.Client.KyvernoV1alpha1().Policies().UpdateStatus(newPolicy)
	return err
}
