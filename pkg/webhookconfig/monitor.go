package webhookconfig

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/tls"
)

//maxRetryCount defines the max deadline count
const (
	tickerInterval    time.Duration = 30 * time.Second
	idleCheckInterval time.Duration = 60 * time.Second
	idleDeadline      time.Duration = idleCheckInterval * 2
)

// Monitor stores the last webhook request time and monitors registered webhooks.
//
// If a webhook is not received in the idleCheckInterval the monitor triggers a
// change in the Kyverno deployment to force a webhook request. If no requests
// are received after idleDeadline the webhooks are deleted and re-registered.
//
// Webhook configurations are checked every tickerInterval. Currently the check
// only queries for the expected resource name, and does not compare other details
// like the webhook settings.
//
type Monitor struct {
	t   time.Time
	mu  sync.RWMutex
	log logr.Logger
}

//NewMonitor returns a new instance of webhook monitor
func NewMonitor(log logr.Logger) *Monitor {
	monitor := &Monitor{
		t:   time.Now(),
		log: log,
	}

	return monitor
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
func (t *Monitor) Run(register *Register, certRenewer *tls.CertRenewer, eventGen event.Interface, stopCh <-chan struct{}) {
	logger := t.log
	logger.V(4).Info("starting webhook monitor", "interval", idleCheckInterval)
	status := newStatusControl(register.client, eventGen, logger.WithName("WebhookStatusControl"))

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// remove me: leader
			if skipWebhookCheck(register, logger.WithName("statusCheck/skipWebhookCheck")) {
				logger.Info("skip validating webhook status, Kyverno is in rolling update")
				continue
			}

			if err := register.Check(); err != nil {
				t.log.Error(err, "missing webhooks")
				if err := register.Register(); err != nil {
					logger.Error(err, "failed to register webhooks")
				}

				continue
			}

			// remove me: all workers
			timeDiff := time.Since(t.Time())
			if timeDiff > idleDeadline {
				err := fmt.Errorf("admission control configuration error")
				logger.Error(err, "webhook check failed", "deadline", idleDeadline)
				if err := status.failure(); err != nil {
					logger.Error(err, "failed to annotate deployment webhook status to failure")
				}

				if err := register.Register(); err != nil {
					logger.Error(err, "Failed to register webhooks")
				}

				continue
			}

			// remove me: all workers
			if timeDiff > idleCheckInterval {
				logger.V(1).Info("webhook idle time exceeded", "deadline", idleCheckInterval)
				if skipWebhookCheck(register, logger.WithName("skipWebhookCheck")) {
					logger.Info("skip validating webhook status, Kyverno is in rolling update")
					continue
				}

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

// skipWebhookCheck returns true if Kyverno is in rolling update
func skipWebhookCheck(register *Register, logger logr.Logger) bool {
	_, deploy, err := register.GetKubePolicyDeployment()
	if err != nil {
		logger.Info("unable to get Kyverno deployment", "reason", err.Error())
		return false
	}

	return tls.IsKyvernoIsInRollingUpdate(deploy.UnstructuredContent(), logger)
}
