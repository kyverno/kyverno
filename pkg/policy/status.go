package policy

import (
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
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
}

//NewPolicyStatAggregator returns a new policy status
func NewPolicyStatAggregator(client *kyvernoclient.Clientset) *PolicyStatusAggregator {
	psa := PolicyStatusAggregator{
		startTime: time.Now(),
		ch:        make(chan PolicyStat),
	}
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
	for r := range psa.ch {
		glog.V(4).Infof("recieved policy stats %v", r)
	}
}

type PolicyStatusInterface interface {
	SendStat(stat PolicyStat)
	UpdateViolationCount(p *kyverno.Policy, pvList []*kyverno.PolicyViolation) error
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
func (psa *PolicyStatusAggregator) UpdateViolationCount(p *kyverno.Policy, pvList []*kyverno.PolicyViolation) error {
	newStatus := calculateStatus(pvList)
	if reflect.DeepEqual(newStatus, p.Status) {
		// no update to status
		return nil
	}
	// update status
	newPolicy := p
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
