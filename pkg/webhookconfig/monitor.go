package webhookconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
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
// Webhook configurations are checked every tickerInterval by the leader. Currently
// the check only queries for the expected resource name, and does not compare
// other details like the webhook settings.
//
type Monitor struct {
	// lastSeenRequestTime records the timestamp
	// of the latest received admission request
	lastSeenRequestTime time.Time
	mu                  sync.RWMutex

	leaderelection leaderelection.Interface

	log logr.Logger
}

// NewMonitor returns a new instance of webhook monitor
func NewMonitor(kubeClient kubernetes.Interface, log logr.Logger) (*Monitor, error) {
	monitor := &Monitor{
		lastSeenRequestTime: time.Now(),
		log:                 log,
	}

	leader, err := leaderelection.New("webhook-monitor", config.KyvernoNamespace, kubeClient, nil, nil, log)
	if err != nil {
		return nil, errors.Wrapf(err, "error electing leader")
	}

	monitor.leaderelection = leader
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

	go t.leaderelection.Run(context.Background())

	logger.V(4).Info("starting webhook monitor", "interval", idleCheckInterval)
	status := newStatusControl(register, eventGen, t.log.WithName("WebhookStatusControl"))

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:

			if t.leaderelection.IsLeader() {
				err := registerWebhookIfNotPresent(register, t.log.WithName("registerWebhookIfNotPresent"))
				if err != nil {
					t.log.Error(err, "")
				}
			}

			timeDiff := time.Since(t.Time())
			if timeDiff > idleDeadline {
				err := fmt.Errorf("admission control configuration error")
				logger.Error(err, "webhook check failed", "deadline", idleDeadline.String())
				if err := status.failure(); err != nil {
					logger.Error(err, "failed to annotate deployment webhook status to failure")
				}

				if err := register.Register(); err != nil {
					logger.Error(err, "Failed to register webhooks")
				}

				continue
			}

			if timeDiff > idleCheckInterval {
				logger.Info("webhook idle time exceeded", "deadline", idleCheckInterval.String())
				if skipWebhookCheck(register, logger.WithName("skipWebhookCheck")) {
					logger.Info("skip validating webhook status, Kyverno is in rolling update")
					continue
				}

				lastRequestTime := lastRequestTimeFromAnnotation(register, t.log.WithName("lastRequestTimeFromAnnotation"))
				if lastRequestTime == nil {
					now := time.Now()
					lastRequestTime = &now
					// if timestamp from the annotation is older than the lastSeenRequestTime
					// of the current instance, update the annotation
					if err := status.UpdateLastRequestTimestmap(t.Time()); err != nil {
						logger.Error(err, "failed to annotate deployment for webhook status")
					}
					logger.V(2).Info("updated annotation timestamp", "time", lastRequestTime)
				}

				if t.Time().Before(*lastRequestTime) {
					t.SetTime(*lastRequestTime)
					logger.V(2).Info("updated in-memory timestamp", "time", lastRequestTime)
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

	annotation, ok, err := unstructured.NestedStringMap(deploy.UnstructuredContent(), "annotations")
	if err != nil {
		logger.Info("unable to get annotation", "reason", err.Error())
		return nil
	}

	if !ok {
		logger.Info("timestamp not set in the annotation, setting")
		return nil
	}

	timeStamp := annotation[annLastRequestTime]
	annTime, err := time.Parse(time.RFC3339, timeStamp)
	if err != nil {
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
