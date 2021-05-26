package webhookconfig

import (
	"context"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/tls"
	ktls "github.com/kyverno/kyverno/pkg/tls"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Interface interface {
	// Run starts the certManager
	Run()

	// LeaderElection returns the factory for leaderElection
	LeaderElection() leaderelection.Interface

	// InitTLSPemPair initializes the TLSPemPair
	// it should be invoked by the leader
	InitTLSPemPair()

	// GetTLSPemPair gets the existing TLSPemPair from the secret
	GetTLSPemPair() (*ktls.PemPair, error)
}
type certManager struct {
	leaderElection leaderelection.Interface
	renewer        *tls.CertRenewer
	secretQueue    chan bool
	stopCh         <-chan struct{}
	log            logr.Logger
}

func NewCertManager(secretInformer informerv1.SecretInformer, kubeClient kubernetes.Interface, certRenewer *tls.CertRenewer, log logr.Logger, stopCh <-chan struct{}) (Interface, error) {
	manager := &certManager{
		renewer:     certRenewer,
		secretQueue: make(chan bool, 1),
		stopCh:      stopCh,
		log:         log,
	}

	var err error
	f := func() { manager.InitTLSPemPair() }
	manager.leaderElection, err = leaderelection.New("cert-manager", config.KyvernoNamespace, kubeClient, f, nil, nil, log.WithName("LeaderElection"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to elector leader")
	}

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addSecretFunc,
		UpdateFunc: manager.updateSecretFunc,
	})

	go manager.leaderElection.Run(context.Background())
	return manager, nil
}

func (m *certManager) addSecretFunc(obj interface{}) {
	if !m.leaderElection.IsLeader() {
		m.log.V(3).Info("skip enqueuing secret for non-leader", "instance", m.leaderElection.ID())
		return
	}

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

func (m *certManager) updateSecretFunc(oldObj interface{}, newObj interface{}) {
	if !m.leaderElection.IsLeader() {
		m.log.V(3).Info("skip enqueuing secret for non-leader", "instance", m.leaderElection.ID())
		return
	}

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
	m.log.V(4).Info("secret updated, reconciling webhook configurations")
}

func (m *certManager) LeaderElection() leaderelection.Interface {
	return m.leaderElection
}

func (m *certManager) InitTLSPemPair() {
	if !m.leaderElection.IsLeader() {
		m.log.Error(errors.Errorf("illegal call: only the leader can init the TLS PemPair"), "instance", m.leaderElection.ID())
		return
	}

	_, err := m.renewer.InitTLSPemPair()
	if err != nil {
		m.log.Error(err, "initialaztion error, instance: %s", m.leaderElection.ID())
		os.Exit(1)
	}
}

func (m *certManager) GetTLSPemPair() (*ktls.PemPair, error) {
	var tls *ktls.PemPair
	var err error

	retryReadTLS := func() error {
		tls, err = ktls.ReadTLSPair(m.renewer.ClientConfig(), m.renewer.Client())
		return err
	}

	f := common.RetryFunc(time.Second, time.Minute, retryReadTLS, m.log.WithName("GetTLSPemPair/Retry"))
	f()

	return tls, err
}

func (m *certManager) Run() {
	if !m.leaderElection.IsLeader() {
		m.log.V(2).Info("skip enqueuing secret for non-leader", "instance", m.leaderElection.ID())
		return
	}

	m.log.Info("start managing certificate")
	certsRenewalTicker := time.NewTicker(tls.CertRenewalInterval)
	defer certsRenewalTicker.Stop()

	for {
		select {
		case <-certsRenewalTicker.C:
			valid, err := m.renewer.ValidCert()
			if err != nil {
				m.log.Error(err, "failed to validate cert")

				if !strings.Contains(err.Error(), tls.ErrorsNotFound) {
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

				if !strings.Contains(err.Error(), tls.ErrorsNotFound) {
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
