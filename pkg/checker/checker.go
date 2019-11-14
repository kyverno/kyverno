package checker

import (
	"sync"
	"time"

	"github.com/golang/glog"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"k8s.io/apimachinery/pkg/labels"
)

//MaxRetryCount defines the max deadline count
const MaxRetryCount int = 3

// LastReqTime
type LastReqTime struct {
	t  time.Time
	mu sync.RWMutex
}

func (t *LastReqTime) Time() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.t
}

func (t *LastReqTime) SetTime(tm time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.t = tm
}

func NewLastReqTime() *LastReqTime {
	return &LastReqTime{
		t: time.Now(),
	}
}

func checkIfPolicyWithMutateAndGenerateExists(pLister kyvernolister.ClusterPolicyLister) bool {
	policies, err := pLister.ListResources(labels.NewSelector())
	if err != nil {
		glog.Error()
	}
	for _, policy := range policies {
		if policy.HasMutateOrValidate() {
			// as there exists one policy with mutate or validate rule
			// so there must be a webhook configuration on resource
			return true
		}
	}
	return false
}

//Run runs the checker and verify the resource update
func (t *LastReqTime) Run(pLister kyvernolister.ClusterPolicyLister,eventGen event.Interface, client *dclient.Client, defaultResync time.Duration, deadline time.Duration, stopCh <-chan struct{}) {
	glog.V(2).Infof("starting default resync for webhook checker with resync time %d", defaultResync)
	maxDeadline := deadline * time.Duration(MaxRetryCount)
	ticker := time.NewTicker(defaultResync)
	var statuscontrol StatusInterface
	/// interface to update and increment kyverno webhook status via annotations
	statuscontrol = NewVerifyControl(client,eventGen)
	// send the initial update status
	if checkIfPolicyWithMutateAndGenerateExists(pLister) {
		if err := statuscontrol.SuccessStatus(); err != nil {
			glog.Error(err)
		}
	}

	defer ticker.Stop()
	// - has recieved request ->  set webhookstatus as "True"
	// - no requests recieved
	// 						  -> if greater than deadline, send update request
	// 						  -> if greater than maxDeadline, send failed status update
	for {
		select {
		case <-ticker.C:
			// if there are no policies then we dont have a webhook on resource.
			// we indirectly check if the resource
			if !checkIfPolicyWithMutateAndGenerateExists(pLister) {
				continue
			}
			// get current time
			timeDiff := time.Since(t.Time())
			if timeDiff > maxDeadline {
				glog.Infof("failed to recieve any request for more than %v ", maxDeadline)
				glog.Info("Admission Control failing: Webhook is not recieving requests forwarded by api-server as per webhook configurations")
				// set the status unavailable
				if err := statuscontrol.FailedStatus(); err != nil {
					glog.Error(err)
				}
				continue
			}
			if timeDiff > deadline {
				glog.Info("Admission Control failing: Webhook is not recieving requests forwarded by api-server as per webhook configurations")
				// send request to update the kyverno deployment
				if err := statuscontrol.IncrementAnnotation(); err != nil {
					glog.Error(err)
				}
				continue
			}
			// if the status was false before then we update it to true
			// send request to update the kyverno deployment
			if err := statuscontrol.SuccessStatus(); err != nil {
				glog.Error(err)
			}
		case <-stopCh:
			// handler termination signal
			glog.V(2).Infof("stopping default resync for webhook checker")
			return
		}
	}
}
