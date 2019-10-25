package webhooks

import (
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
)

const MaxRetryCount int = 3

// Last Request Time
type LastReqTime struct {
	t          time.Time
	mu         sync.RWMutex
	RetryCount int
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
	t.RetryCount = MaxRetryCount
}

func (t *LastReqTime) DecrementRetryCounter() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.RetryCount--
}

func NewLastReqTime() *LastReqTime {
	return &LastReqTime{
		t: time.Now(),
	}
}

func (t *LastReqTime) checker(kyvernoClient *kyvernoclient.Clientset, defaultResync time.Duration, deadline time.Duration, stopCh <-chan struct{}) {
	sendDummyRequest := func(kyvernoClient *kyvernoclient.Clientset) {
		dummyPolicy := kyverno.ClusterPolicy{
			Spec: kyverno.Spec{
				Rules: []kyverno.Rule{
					kyverno.Rule{
						Name: "dummyPolicy",
						MatchResources: kyverno.MatchResources{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"Deployment"},
							},
						},
						Validation: kyverno.Validation{
							Message: "dummy validation policy rule",
							Pattern: "dummypattern",
						},
					},
				},
			},
		}
		// this
		kyvernoClient.KyvernoV1alpha1().ClusterPolicies().Create(&dummyPolicy)
	}
	glog.V(2).Infof("starting default resync for webhook checker with resync time %d", defaultResync)
	ticker := time.NewTicker(defaultResync)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// get current time
			timeDiff := time.Since(t.Time())
			if timeDiff > deadline {
				if t.RetryCount == 0 {
					// set the status unavailable
				}
				t.DecrementRetryCounter()
				// send request again
			}

		case <-stopCh:
			// handler termination signal
			break
		}
	}
	glog.V(2).Info("stopping default resync for webhook checker")
}
