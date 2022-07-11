package webhookconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
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
	// leaseClient is used to manage Kyverno lease
	leaseClient coordinationv1.LeaseInterface

	// lastSeenRequestTime records the timestamp
	// of the latest received admission request
	lastSeenRequestTime time.Time
	mu                  sync.RWMutex

	log logr.Logger
}

// NewMonitor returns a new instance of webhook monitor
func NewMonitor(kubeClient kubernetes.Interface, log logr.Logger) (*Monitor, error) {
	monitor := &Monitor{
		leaseClient:         kubeClient.CoordinationV1().Leases(config.KyvernoNamespace),
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
	logger := t.log.WithName("webhookMonitor")

	logger.V(3).Info("starting webhook monitor", "interval", idleCheckInterval.String())
	status := newStatusControl(t.leaseClient, eventGen, logger.WithName("WebhookStatusControl"))

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	createDefaultWebhook := register.createDefaultWebhook
	for {
		select {
		case webhookKind := <-createDefaultWebhook:
			logger.Info("received recreation request for resource webhook")
			if webhookKind == kindMutating {
				err := register.createResourceMutatingWebhookConfiguration(register.readCaData())
				if err != nil {
					logger.Error(err, "failed to create default MutatingWebhookConfiguration for resources, the webhook will be reconciled", "interval", tickerInterval)
				}
			} else if webhookKind == kindValidating {
				err := register.createResourceValidatingWebhookConfiguration(register.readCaData())
				if err != nil {
					logger.Error(err, "failed to create default ValidatingWebhookConfiguration for resources, the webhook will be reconciled", "interval", tickerInterval)
				}
			}

		case <-ticker.C:

			err := registerWebhookIfNotPresent(register, t.log.WithName("registerWebhookIfNotPresent"))
			if err != nil {
				t.log.Error(err, "")
			}

			// update namespaceSelector every 30 seconds
			go func() {
				if register.autoUpdateWebhooks {
					select {
					case register.UpdateWebhookChan <- true:
						logger.V(4).Info("updating webhook configurations for namespaceSelector with latest kyverno ConfigMap")
					default:
						logger.V(4).Info("skipped sending update webhook signal as the channel was blocking")
					}
				}
			}()

			timeDiff := time.Since(t.Time())
			lastRequestTimeFromAnn := lastRequestTimeFromAnnotation(t.leaseClient, t.log.WithName("lastRequestTimeFromAnnotation"))
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
				err := fmt.Errorf("webhook hasn't received requests in %v, updating Kyverno to verify webhook status", idleDeadline.String())
				logger.Error(err, "webhook check failed", "time", t.Time(), "lastRequestTimestamp", lastRequestTimeFromAnn)

				// update deployment to renew lastSeenRequestTime
				if err := status.failure(); err != nil {
					logger.Error(err, "failed to annotate deployment webhook status to failure")

					if err := register.Register(); err != nil {
						logger.Error(err, "Failed to register webhooks")
					}
				}

				continue

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
					logger.V(3).Info("updating annotation lastRequestTimestamp with the latest in-memory timestamp", "time", t.Time(), "lastRequestTimestamp", lastRequestTimeFromAnn)
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

func lastRequestTimeFromAnnotation(leaseClient coordinationv1.LeaseInterface, logger logr.Logger) *time.Time {

	lease, err := leaseClient.Get(context.TODO(), "kyverno", metav1.GetOptions{})
	if err != nil {
		logger.Info("Lease 'kyverno' not found. Starting clean-up...")
	}

	timeStamp := lease.GetAnnotations()
	if timeStamp == nil {
		logger.Info("timestamp not set in the annotation, setting")
		return nil
	}

	annTime, err := time.Parse(time.RFC3339, timeStamp[annLastRequestTime])
	if err != nil {
		logger.Error(err, "failed to parse timestamp annotation", "timeStamp", timeStamp[annLastRequestTime])
		return nil
	}

	return &annTime
}

// skipWebhookCheck returns true if Kyverno is in rolling update
func skipWebhookCheck(register *Register, logger logr.Logger) bool {
	deploy, err := register.GetKubePolicyDeployment()
	if err != nil {
		logger.Info("unable to get Kyverno deployment", "reason", err.Error())
		return false
	}

	return tls.IsKyvernoInRollingUpdate(deploy, logger)
}
