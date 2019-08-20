package policy

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

// type PolicyStatus struct {
// 	// average time required to process the policy rules on a resource
// 	avgExecutionTime time.Duration
// 	// Count of rules that were applied succesfully
// 	rulesAppliedCount int
// 	// Count of resources for whom update/create api requests were blocked as the resoruce did not satisfy the policy rules
// 	resourcesBlockedCount int
// 	// Count of the resource for whom the mutation rules were applied succesfully
// 	resourcesMutatedCount int
// }

//PolicyStatusAggregator stores information abt aggregation
type PolicyStatusAggregator struct {
	// time since we start aggregating the stats
	startTime time.Time
	// channel to recieve stats
	ch chan PolicyStat
	// update policy status
	psControl PStatusControlInterface
	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.PolicyLister
	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced
	// UpdateViolationCount and SendStat can update same policy status
	// we need to sync the updates using policyName
	policyUpdateData sync.Map
}

//NewPolicyStatAggregator returns a new policy status
func NewPolicyStatAggregator(client *kyvernoclient.Clientset, pInformer kyvernoinformer.PolicyInformer) *PolicyStatusAggregator {
	psa := PolicyStatusAggregator{
		startTime: time.Now(),
		ch:        make(chan PolicyStat),
	}
	psa.pLister = pInformer.Lister()
	psa.pListerSynced = pInformer.Informer().HasSynced
	psa.psControl = PSControl{Client: client}
	//TODO: add WaitGroup
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
			glog.Infof("Failed to update stats for policy %s: %v", r.PolicyName, err)
		}
	}

}

func (psa *PolicyStatusAggregator) updateStats(stats PolicyStat) error {
	func() {
		glog.V(4).Infof("lock updates for policy %s", stats.PolicyName)
		// Lock the update for policy
		psa.policyUpdateData.Store(stats.PolicyName, struct{}{})
	}()
	defer func() {
		glog.V(4).Infof("Unlock updates for policy %s", stats.PolicyName)
		psa.policyUpdateData.Delete(stats.PolicyName)
	}()

	// //wait for cache sync
	// if !cache.WaitForCacheSync(nil, psa.pListerSynced) {
	// 	glog.Infof("unable to sync cache for policy informer")
	// 	return nil
	// }
	// get policy
	policy, err := psa.pLister.Get(stats.PolicyName)
	if err != nil {
		glog.V(4).Infof("failed to get policy %s. Unable to update violation count: %v", stats.PolicyName, err)
		return err
	}
	newpolicy := policy
	fmt.Println(newpolicy.ResourceVersion)
	newpolicy.Status = kyverno.PolicyStatus{}
	glog.V(4).Infof("updating stats for policy %s", policy.Name)
	// rules applied count
	newpolicy.Status.RulesAppliedCount = newpolicy.Status.RulesAppliedCount + stats.RulesAppliedCount
	// resource blocked count
	if stats.ResourceBlocked {
		policy.Status.ResourcesBlockedCount++
	}
	var zeroDuration time.Duration
	if newpolicy.Status.AvgExecutionTimeMutation != zeroDuration {
		// avg execution time for mutation rules
		newpolicy.Status.AvgExecutionTimeMutation = (newpolicy.Status.AvgExecutionTimeMutation + stats.MutationExecutionTime) / 2
	} else {
		newpolicy.Status.AvgExecutionTimeMutation = stats.MutationExecutionTime
	}
	if policy.Status.AvgExecutionTimeValidation != zeroDuration {
		// avg execution time for validation rules
		newpolicy.Status.AvgExecutionTimeValidation = (newpolicy.Status.AvgExecutionTimeValidation + stats.ValidationExecutionTime) / 2
	} else {
		newpolicy.Status.AvgExecutionTimeValidation = stats.ValidationExecutionTime
	}
	return psa.psControl.UpdatePolicyStatus(newpolicy)
}

//PolicyStatusInterface provides methods to modify policyStatus
type PolicyStatusInterface interface {
	SendStat(stat PolicyStat)
	UpdateViolationCount(policyName string, pvList []*kyverno.PolicyViolation) error
}

//PolicyStat stored stats for policy
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
		glog.V(4).Infof("lock updates for policy %s", policyName)
		// Lock the update for policy
		psa.policyUpdateData.Store(policyName, struct{}{})
	}()
	defer func() {
		glog.V(4).Infof("Unlock updates for policy %s", policyName)
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
		glog.V(4).Infof("no changes in policy violation count for policy %s", policy.Name)
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
		ViolationCount: violationCount,
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

//PSControl allows update for policy status
type PSControl struct {
	Client kyvernoclient.Interface
}

//UpdatePolicyStatus update policy status
func (c PSControl) UpdatePolicyStatus(newPolicy *kyverno.Policy) error {
	_, err := c.Client.KyvernoV1alpha1().Policies().UpdateStatus(newPolicy)
	return err
}
