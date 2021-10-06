package webhookconfig

import (
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	ktls "github.com/kyverno/kyverno/pkg/tls"
	v1 "k8s.io/api/core/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Interface interface {
	// Run starts the certManager
	Run(stopCh <-chan struct{})

	// InitTLSPemPair initializes the TLSPemPair
	// it should be invoked by the leader
	InitTLSPemPair()

	// GetTLSPemPair gets the existing TLSPemPair from the secret
	GetTLSPemPair() (*ktls.PemPair, error)
}
type certManager struct {
	renewer        *ktls.CertRenewer
	secretInformer informerv1.SecretInformer
	secretQueue    chan bool
	stopCh         <-chan struct{}
	log            logr.Logger
}

func NewCertManager(secretInformer informerv1.SecretInformer, kubeClient kubernetes.Interface, certRenewer *ktls.CertRenewer, log logr.Logger, stopCh <-chan struct{}) (Interface, error) {
	manager := &certManager{
		renewer:        certRenewer,
		secretInformer: secretInformer,
		secretQueue:    make(chan bool, 1),
		stopCh:         stopCh,
		log:            log,
	}

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addSecretFunc,
		UpdateFunc: manager.updateSecretFunc,
	})

	return manager, nil
}

func (m *certManager) addSecretFunc(obj interface{}) {
	secret := obj.(*v1.Secret)
	if secret.GetNamespace() != config.KyvernoNamespace {
		return
	}

	val, ok := secret.GetAnnotations()[ktls.SelfSignedAnnotation]
	if !ok || val != "true" {
		return
	}

	m.secretQueue <- true
}

func (m *certManager) updateSecretFunc(oldObj interface{}, newObj interface{}) {
	old := oldObj.(*v1.Secret)
	new := newObj.(*v1.Secret)
	if new.GetNamespace() != config.KyvernoNamespace {
		return
	}

	val, ok := new.GetAnnotations()[ktls.SelfSignedAnnotation]
	if !ok || val != "true" {
		return
	}

	if reflect.DeepEqual(old.DeepCopy().Data, new.DeepCopy().Data) {
		return
	}

	m.secretQueue <- true
	m.log.V(4).Info("secret updated, reconciling webhook configurations")
}

func (m *certManager) InitTLSPemPair() {
	_, err := m.renewer.InitTLSPemPair()
	if err != nil {
		m.log.Error(err, "initialization error")
		os.Exit(1)
	}
}

func (m *certManager) GetTLSPemPair() (*ktls.PemPair, error) {
	var tls *ktls.PemPair
	var err error

	retryReadTLS := func() error {
		tls, err = ktls.ReadTLSPair(m.renewer.ClientConfig(), m.renewer.Client())
		if err != nil {
			return err
		}

		m.log.Info("read TLS pem pair from the secret")
		return nil
	}

	f := common.RetryFunc(time.Second, time.Minute, retryReadTLS, m.log.WithName("GetTLSPemPair/Retry"))
	err = f()

	return tls, err
}

func (m *certManager) Run(stopCh <-chan struct{}) {
	if !cache.WaitForCacheSync(stopCh, m.secretInformer.Informer().HasSynced) {
		m.log.Info("failed to sync informer cache")
		return
	}

	m.secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.addSecretFunc,
		UpdateFunc: m.updateSecretFunc,
	})

	m.log.Info("start managing certificate")
	certsRenewalTicker := time.NewTicker(ktls.CertRenewalInterval)
	defer certsRenewalTicker.Stop()

	for {
		select {
		case <-certsRenewalTicker.C:
			valid, err := m.renewer.ValidCert()
			if err != nil {
				m.log.Error(err, "failed to validate cert")

				if !strings.Contains(err.Error(), ktls.ErrorsNotFound) {
					continue
				}
			}

			if valid {
				continue
			}

			m.log.Info("rootCA is about to expire, trigger a rolling update to renew the cert")
			if err := m.renewer.RollingUpdate(); err != nil {
				m.log.Error(err, "unable to trigger a rolling update to renew rootCA, force restarting")
				os.Exit(1)
			}

		case <-m.secretQueue:
			valid, err := m.renewer.ValidCert()
			if err != nil {
				m.log.Error(err, "failed to validate cert")

				if !strings.Contains(err.Error(), ktls.ErrorsNotFound) {
					continue
				}
			}

			if valid {
				continue
			}

			m.log.Info("rootCA has changed, updating webhook configurations")
			if err := m.renewer.RollingUpdate(); err != nil {
				m.log.Error(err, "unable to trigger a rolling update to re-register webhook server, force restarting")
				os.Exit(1)
			}

		case <-m.stopCh:
			m.log.V(2).Info("stopping cert renewer")
			return
		}
	}
}
