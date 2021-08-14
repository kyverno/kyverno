package webhookconfig

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

//maxRetryCount defines the max deadline count
const (
	tickerInterval    time.Duration = 30 * time.Second
	idleCheckInterval time.Duration = 60 * time.Second
	idleDeadline      time.Duration = idleCheckInterval * 5
)

// Monitor stores the last webhook request time and monitors registered webhooks.
//
// If a webhook is not received in the idleCheckInterval the monitor triggers a
// change in the Kyverno deployment to force a webhook request. If no requests
// are received after idleDeadline the webhooks are deleted and re-registered.
//
// Each instance has an in-memory flag lastSeenRequestTime, recording the last
// received admission timestamp by the current instance. And the latest timestamp
// (latestTimestamp) is recorded in the annotation of the Kyverno deployment,
// this annotation could be updated by any instance. If the duration from
// latestTimestamp is longer than idleCheckInterval, the monitor triggers an
// annotation update; otherwise lastSeenRequestTime is updated to latestTimestamp.
//
//
// Webhook configurations are checked every tickerInterval across all instances.
// Currently the check only queries for the expected resource name, and does
// not compare other details like the webhook settings.
//
type Monitor struct {
	// lastSeenRequestTime records the timestamp
	// of the latest received admission request
	lastSeenRequestTime time.Time
	mu                  sync.RWMutex

	log logr.Logger
}

// NewMonitor returns a new instance of webhook monitor
func NewMonitor(kubeClient kubernetes.Interface, log logr.Logger) (*Monitor, error) {
	monitor := &Monitor{
		lastSeenRequestTime: time.Now(),
		log:                 log,
	}

	return monitor, nil
}

// Time returns the last request time
func (t *Monitor) Time() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastSeenRequestTime
}

// SetTime updates the last request time
func (t *Monitor) SetTime(tm time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastSeenRequestTime = tm
}

// Run runs the checker and verify the resource update
func (t *Monitor) Run(register *Register, certRenewer *tls.CertRenewer, eventGen event.Interface, stopCh <-chan struct{}) {
	logger := t.log

	logger.V(4).Info("starting webhook monitor", "interval", idleCheckInterval.String())
	status := newStatusControl(register, eventGen, t.log.WithName("WebhookStatusControl"))

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:

			err := registerWebhookIfNotPresent(register, t.log.WithName("registerWebhookIfNotPresent"))
			if err != nil {
				t.log.Error(err, "")
			}

			timeDiff := time.Since(t.Time())
			lastRequestTimeFromAnn := lastRequestTimeFromAnnotation(register, t.log.WithName("lastRequestTimeFromAnnotation"))
			if lastRequestTimeFromAnn == nil {
				if err := status.UpdateLastRequestTimestmap(t.Time()); err != nil {
					logger.Error(err, "failed to annotate deployment for lastRequestTime")
				} else {
					logger.Info("initialized lastRequestTimestamp", "time", t.Time())
				}
				continue
			}

			switch {
			case timeDiff > idleDeadline:
				err := fmt.Errorf("admission control configuration error")
				logger.Error(err, "webhook check failed", "deadline", idleDeadline.String())
				if err := status.failure(); err != nil {
					logger.Error(err, "failed to annotate deployment webhook status to failure")
				}

				if err := register.Register(); err != nil {
					logger.Error(err, "Failed to register webhooks")
				}

			case timeDiff > 2*idleCheckInterval:
				if skipWebhookCheck(register, logger.WithName("skipWebhookCheck")) {
					logger.Info("skip validating webhook status, Kyverno is in rolling update")
					continue
				}

				if t.Time().Before(*lastRequestTimeFromAnn) {
					t.SetTime(*lastRequestTimeFromAnn)
					logger.V(3).Info("updated in-memory timestamp", "time", lastRequestTimeFromAnn)
				}
			}

			idleT := time.Since(*lastRequestTimeFromAnn)
			if idleT > idleCheckInterval {
				if t.Time().After(*lastRequestTimeFromAnn) {
					logger.V(3).Info("updating annotation lastRequestTimestamp with the latest in-memory timestamp", "time", t.Time())
					if err := status.UpdateLastRequestTimestmap(t.Time()); err != nil {
						logger.Error(err, "failed to update lastRequestTimestamp annotation")
					}
				}
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

func registerWebhookIfNotPresent(register *Register, logger logr.Logger) error {
	if skipWebhookCheck(register, logger.WithName("skipWebhookCheck")) {
		logger.Info("skip validating webhook status, Kyverno is in rolling update")
		return nil
	}

	if err := register.Check(); err != nil {
		logger.Error(err, "missing webhooks")

		if err := register.Register(); err != nil {
			return errors.Wrap(err, "failed to register webhooks")
		}
	}

	return nil
}

func lastRequestTimeFromAnnotation(register *Register, logger logr.Logger) *time.Time {
	_, deploy, err := register.GetKubePolicyDeployment()
	if err != nil {
		logger.Info("unable to get Kyverno deployment", "reason", err.Error())
		return nil
	}

	timeStamp, ok, err := unstructured.NestedString(deploy.UnstructuredContent(), "metadata", "annotations", annLastRequestTime)
	if err != nil {
		logger.Info("unable to get annotation", "reason", err.Error())
		return nil
	}

	if !ok {
		logger.Info("timestamp not set in the annotation, setting")
		return nil
	}

	annTime, err := time.Parse(time.RFC3339, timeStamp)
	if err != nil {
		logger.Error(err, "failed to parse timestamp annotation", "timeStamp", timeStamp)
		return nil
	}

	return &annTime
}

// skipWebhookCheck returns true if Kyverno is in rolling update
func skipWebhookCheck(register *Register, logger logr.Logger) bool {
	_, deploy, err := register.GetKubePolicyDeployment()
	if err != nil {
		logger.Info("unable to get Kyverno deployment", "reason", err.Error())
		return false
	}

	return tls.IsKyvernoInRollingUpdate(deploy.UnstructuredContent(), logger)
}
