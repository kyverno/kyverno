package webhookconfig

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/tls"
	v1 "k8s.io/api/core/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
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
	t           time.Time
	mu          sync.RWMutex
	secretQueue chan bool
	log         logr.Logger
}

//NewMonitor returns a new instance of webhook monitor
func NewMonitor(secretInformer informerv1.SecretInformer, log logr.Logger) *Monitor {
	monitor := &Monitor{
		t:           time.Now(),
		secretQueue: make(chan bool, 1),
		log:         log,
	}

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    monitor.addSecretFunc,
		UpdateFunc: monitor.updateSecretFunc,
	})

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

func (t *Monitor) addSecretFunc(obj interface{}) {
	secret := obj.(*v1.Secret)
	if secret.GetNamespace() != config.KyvernoNamespace {
		return
	}

	val, ok := secret.GetAnnotations()[tls.SelfSignedAnnotation]
	if !ok || val != "true" {
		return
	}

	t.secretQueue <- true
}

func (t *Monitor) updateSecretFunc(oldObj interface{}, newObj interface{}) {
	old := oldObj.(*v1.Secret)
	new := newObj.(*v1.Secret)
	if new.GetNamespace() != config.KyvernoNamespace {
		return
	}

	val, ok := new.GetAnnotations()[tls.SelfSignedAnnotation]
	if !ok || val != "true" {
		return
	}

	if reflect.DeepEqual(old.DeepCopy().Data, new.DeepCopy().Data) {
		return
	}

	t.secretQueue <- true
	t.log.V(4).Info("secret updated, reconciling webhook configurations")
}

//Run runs the checker and verify the resource update
func (t *Monitor) Run(register *Register, certRenewer *tls.CertRenewer, eventGen event.Interface, stopCh <-chan struct{}) {
	logger := t.log
	logger.V(4).Info("starting webhook monitor", "interval", idleCheckInterval)
	status := newStatusControl(register.client, eventGen, logger.WithName("WebhookStatusControl"))

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	certsRenewalTicker := time.NewTicker(tls.CertRenewalInterval)
	defer certsRenewalTicker.Stop()

	for {
		select {
		case <-ticker.C:
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

		case <-certsRenewalTicker.C:
			valid, err := certRenewer.ValidCert()
			if err != nil {
				logger.Error(err, "failed to validate cert")

				if !strings.Contains(err.Error(), tls.ErrorsNotFound) {
					continue
				}
			}

			if valid {
				continue
			}

			logger.Info("rootCA is about to expire, trigger a rolling update to renew the cert")
			if err := certRenewer.RollingUpdate(); err != nil {
				logger.Error(err, "unable to trigger a rolling update to renew rootCA, force restarting")
				os.Exit(1)
			}

		case <-t.secretQueue:
			valid, err := certRenewer.ValidCert()
			if err != nil {
				logger.Error(err, "failed to validate cert")

				if !strings.Contains(err.Error(), tls.ErrorsNotFound) {
					continue
				}
			}

			if valid {
				continue
			}

			logger.Info("rootCA has changed, updating webhook configurations")
			if err := certRenewer.RollingUpdate(); err != nil {
				logger.Error(err, "unable to trigger a rolling update to re-register webhook server, force restarting")
				os.Exit(1)
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
