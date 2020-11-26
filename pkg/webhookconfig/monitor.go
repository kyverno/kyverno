package webhookconfig

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
)

//maxRetryCount defines the max deadline count
const (
	maxRetryCount   int           = 3
	defaultInterval time.Duration = 60 * time.Second
)

// Monitor stores the last request times for incoming api-requests
// and monitors registered webhooks
type Monitor struct {
	t   time.Time
	mu  sync.RWMutex
	log logr.Logger
}

//NewMonitor returns a new instance of LastRequestTime store
func NewMonitor(log logr.Logger) *Monitor {
	return &Monitor{
		t:   time.Now(),
		log: log,
	}
}

//Time returns the last request time
func (t *Monitor) Time() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.t
}

//SetTime updates the last request time
func (t *Monitor) SetTime(tm time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.t = tm
}

//Run runs the checker and verify the resource update
func (t *Monitor) Run(register *Register, eventGen event.Interface, client *dclient.Client, stopCh <-chan struct{}) {
	logger := t.log
	logger.V(4).Info("starting webhook monitor", "interval", defaultInterval)
	maxDeadline := defaultInterval * time.Duration(maxRetryCount)
	status := newStatusControl(client, eventGen, logger.WithName("WebhookStatusControl"))

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:

			if err := register.Check(); err != nil {
				t.log.Error(err,"missing webhooks")
				if err := register.Register(); err != nil {
					logger.Error(err,"failed to register webhooks")
				}

				continue
			}

			timeDiff := time.Since(t.Time())
			if timeDiff > maxDeadline {
				err := fmt.Errorf("admission control configuration error")
				logger.Error(err, "webhook check failed", "deadline", maxDeadline)
				if err := status.failure(); err != nil {
					logger.Error(err, "failed to annotate deployment webhook status to failure")
				}

				cleanUp := make(chan struct{})
				register.Remove(cleanUp)
				<-cleanUp

				register.Register()
				continue
			}

			if timeDiff > defaultInterval {
				logger.V(3).Info("webhook check deadline exceeded", "deadline", defaultInterval)

				// send request to update the Kyverno deployment
				if err := status.IncrementAnnotation(); err != nil {
					logger.Error(err, "failed to annotate deployment for webhook status")
				}

				continue
			}

			// if the status was false before then we update it to true
			// send request to update the Kyverno deployment
			if err := status.success(); err != nil {
				logger.Error(err, "failed to annotate deployment webhook status to success")
			}

		case <-stopCh:
			// handler termination signal
			logger.V(2).Info("stopping webhook monitor")
			return
		}
	}
}
