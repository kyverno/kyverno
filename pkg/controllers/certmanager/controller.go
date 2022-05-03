package certmanager

import (
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tls"
	v1 "k8s.io/api/core/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Controller interface {
	// Run starts the certManager
	Run(stopCh <-chan struct{})

	// InitTLSPemPair initializes the TLSPemPair
	// it should be invoked by the leader
	InitTLSPemPair()

	// GetTLSPemPair gets the existing TLSPemPair from the secret
	GetTLSPemPair() (*tls.PemPair, error)
}

type controller struct {
	renewer        *tls.CertRenewer
	secretInformer informerv1.SecretInformer
	secretQueue    chan bool
}

func NewController(secretInformer informerv1.SecretInformer, kubeClient kubernetes.Interface, certRenewer *tls.CertRenewer) (Controller, error) {
	manager := &controller{
		renewer:        certRenewer,
		secretInformer: secretInformer,
		secretQueue:    make(chan bool, 1),
	}
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addSecretFunc,
		UpdateFunc: manager.updateSecretFunc,
	})
	return manager, nil
}

func (m *controller) addSecretFunc(obj interface{}) {
	secret := obj.(*v1.Secret)
	if secret.GetNamespace() != config.KyvernoNamespace {
		return
	}
	val, ok := secret.GetAnnotations()[tls.SelfSignedAnnotation]
	if !ok || val != "true" {
		return
	}
	m.secretQueue <- true
}

func (m *controller) updateSecretFunc(oldObj interface{}, newObj interface{}) {
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
	m.secretQueue <- true
	logger.V(4).Info("secret updated, reconciling webhook configurations")
}

func (m *controller) InitTLSPemPair() {
	_, err := m.renewer.InitTLSPemPair()
	if err != nil {
		logger.Error(err, "initialization error")
		os.Exit(1)
	}
}

func (m *controller) GetTLSPemPair() (*tls.PemPair, error) {
	var keyPair *tls.PemPair
	var err error
	retryReadTLS := func() error {
		keyPair, err = tls.ReadTLSPair(m.renewer.ClientConfig(), m.renewer.Client())
		if err != nil {
			return err
		}
		logger.Info("read TLS pem pair from the secret")
		return nil
	}
	msg := "failed to read TLS pair"
	f := common.RetryFunc(time.Second, time.Minute, retryReadTLS, msg, logger.WithName("GetTLSPemPair/Retry"))
	return keyPair, f()
}

func (m *controller) Run(stopCh <-chan struct{}) {
	logger.Info("start managing certificate")
	certsRenewalTicker := time.NewTicker(tls.CertRenewalInterval)
	defer certsRenewalTicker.Stop()
	for {
		select {
		case <-certsRenewalTicker.C:
			valid, err := m.renewer.ValidCert()
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
			if err := m.renewer.RollingUpdate(); err != nil {
				logger.Error(err, "unable to trigger a rolling update to renew rootCA, force restarting")
				os.Exit(1)
			}
		case <-m.secretQueue:
			valid, err := m.renewer.ValidCert()
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
			if err := m.renewer.RollingUpdate(); err != nil {
				logger.Error(err, "unable to trigger a rolling update to re-register webhook server, force restarting")
				os.Exit(1)
			}
		case <-stopCh:
			logger.V(2).Info("stopping cert renewer")
			return
		}
	}
}
