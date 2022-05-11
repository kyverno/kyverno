package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// CertRenewalInterval is the renewal interval for rootCA
	CertRenewalInterval time.Duration = 12 * time.Hour
	// CertValidityDuration is the valid duration for a new cert
	CertValidityDuration time.Duration = 365 * 24 * time.Hour
	// ManagedByLabel is added to Kyverno managed secrets
	ManagedByLabel          string = "cert.kyverno.io/managed-by"
	RootCAKey               string = "rootCA.crt"
	rollingUpdateAnnotation string = "update.kyverno.io/force-rolling-update"
)

// CertRenewer creates rootCA and pem pair to register
// webhook configurations and webhook server
// renews RootCA at the given interval
type CertRenewer struct {
	client               kubernetes.Interface
	certRenewalInterval  time.Duration
	certValidityDuration time.Duration
	certProps            *certificateProps

	// IP address where Kyverno controller runs. Only required if out-of-cluster.
	serverIP string

	log logr.Logger
}

// NewCertRenewer returns an instance of CertRenewer
func NewCertRenewer(client kubernetes.Interface, clientConfig *rest.Config, certRenewalInterval, certValidityDuration time.Duration, serverIP string, log logr.Logger) (*CertRenewer, error) {
	certProps, err := newCertificateProps(clientConfig)
	if err != nil {
		return nil, err
	}
	return &CertRenewer{
		client:               client,
		certRenewalInterval:  certRenewalInterval,
		certValidityDuration: certValidityDuration,
		certProps:            certProps,
		serverIP:             serverIP,
		log:                  log,
	}, nil
}

// InitTLSPemPair Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
func (c *CertRenewer) InitTLSPemPair() error {
	logger := c.log.WithName("InitTLSPemPair")
	ca, err := c.getCASecret()
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	tls, err := c.getTLSSecret()
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	// check they are valid
	if ca != nil && tls != nil {
		if validCert(ca, tls, logger) {
			return nil
		}
	}
	// if not valid, check we can renew them
	if !IsSecretManagedByKyverno(ca) || !IsSecretManagedByKyverno(tls) {
		return fmt.Errorf("tls is not valid but certificates are not managed by kyverno, we can't renew them")
	}
	// renew them
	logger.Info("building key/certificate pair for TLS")
	return c.buildTLSPemPairAndWriteToSecrets(c.serverIP)
}

func (c *CertRenewer) RenewCertificates() error {
	return c.InitTLSPemPair()
}

// buildTLSPemPairAndWriteToSecrets Issues TLS certificate for webhook server using self-signed CA cert
// Returns signed and approved TLS certificate in PEM format
func (c *CertRenewer) buildTLSPemPairAndWriteToSecrets(serverIP string) error {
	caCert, err := generateCA(c.certValidityDuration)
	if err != nil {
		return err
	}
	tlsPair, err := generateCert(caCert, c.certProps, serverIP, c.certValidityDuration)
	if err != nil {
		return err
	}
	if err := c.writeCASecret(caCert); err != nil {
		return fmt.Errorf("failed to write CA cert to secret: %v", err)
	}
	if err = c.writeTLSSecret(tlsPair); err != nil {
		return fmt.Errorf("unable to save TLS pair to the cluster: %v", err)
	}
	return nil
}

func (c *CertRenewer) getSecret(name string) (*corev1.Secret, error) {
	if s, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (c *CertRenewer) getCASecret() (*corev1.Secret, error) {
	return c.getSecret(GenerateRootCASecretName())
}

func (c *CertRenewer) getTLSSecret() (*corev1.Secret, error) {
	return c.getSecret(GenerateTLSPairSecretName())
}

func (c *CertRenewer) writeSecret(secret *corev1.Secret, logger logr.Logger) error {
	logger = logger.WithValues("name", secret.GetName(), "namespace", secret.GetNamespace())
	if _, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if _, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
				logger.Error(err, "failed to update secret")
				return err
			} else {
				logger.Info("secret updated")
				return nil
			}
		} else {
			logger.Error(err, "failed to create secret")
			return err
		}
	} else {
		logger.Info("secret created")
		return nil
	}
}

// writeCASecret stores the CA cert in secret
func (c *CertRenewer) writeCASecret(ca *keyPair) error {
	logger := c.log.WithName("writeCASecret")
	secret, err := c.getCASecret()
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get CA secret")
		return err
	}
	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateRootCASecretName(),
				Namespace: config.KyvernoNamespace(),
				Labels: map[string]string{
					ManagedByLabel: "kyverno",
				},
			},
			Type: corev1.SecretTypeTLS,
		}
	}
	secret.Type = corev1.SecretTypeTLS
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       CertificateToPem(ca.cert),
		corev1.TLSPrivateKeyKey: PrivateKeyToPem(ca.key),
	}
	return c.writeSecret(secret, logger)
}

// writeTLSSecret Writes the pair of TLS certificate and key to the specified secret.
func (c *CertRenewer) writeTLSSecret(tls *keyPair) error {
	logger := c.log.WithName("writeTLSSecret")
	secret, err := c.getTLSSecret()
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get TLS secret")
		return err
	}
	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GenerateTLSPairSecretName(),
				Namespace: config.KyvernoNamespace(),
				Labels: map[string]string{
					ManagedByLabel: "kyverno",
				},
			},
			Type: corev1.SecretTypeTLS,
		}
	}
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       CertificateToPem(tls.cert),
		corev1.TLSPrivateKeyKey: PrivateKeyToPem(tls.key),
	}
	return c.writeSecret(secret, logger)
}

// ValidCert validates the CA Cert
func (c *CertRenewer) ValidCert() (bool, error) {
	logger := c.log.WithName("validCert")
	ca, err := c.getCASecret()
	if err != nil {
		logger.Error(err, "unable to read CA secret")
		return false, err
	}
	tls, err := c.getTLSSecret()
	if err != nil {
		logger.Error(err, "unable to read TLS secret")
		return false, err
	}
	return validCert(ca, tls, logger), nil
}

func validCert(caSecret *corev1.Secret, tlsSecret *corev1.Secret, logger logr.Logger) bool {
	caPem := caSecret.Data[corev1.TLSCertKey]
	if len(caPem) == 0 {
		caPem = caSecret.Data[RootCAKey]
	}
	// build cert pool
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPem) {
		logger.Info("bad certificate")
		return false
	}
	// valid PEM pair
	_, err := tls.X509KeyPair(tlsSecret.Data[corev1.TLSCertKey], tlsSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		logger.Error(err, "invalid PEM pair")
		return false
	}
	certPemBlock, _ := pem.Decode(tlsSecret.Data[corev1.TLSCertKey])
	if certPemBlock == nil {
		logger.Error(err, "bad private key")
		return false
	}
	cert, err := x509.ParseCertificate(certPemBlock.Bytes)
	if err != nil {
		logger.Error(err, "failed to parse cert")
		return false
	}
	if _, err = cert.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: time.Now(),
	}); err != nil {
		logger.Error(err, "invalid cert")
		return false
	}
	return true
}

// RollingUpdate triggers a rolling update of Kyverno pod.
// It is used when the rootCA is renewed, the restart of
// Kyverno pod will register webhook server with new cert
func (c *CertRenewer) RollingUpdate() error {
	update := func() error {
		deploy, err := c.client.AppsV1().Deployments(config.KyvernoNamespace()).Get(context.TODO(), config.KyvernoDeploymentName(), metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to find Kyverno")
		}
		if IsKyvernoInRollingUpdate(deploy, c.log) {
			return nil
		}
		if deploy.Spec.Template.Annotations == nil {
			deploy.Spec.Template.Annotations = map[string]string{}
		}
		deploy.Spec.Template.Annotations[rollingUpdateAnnotation] = time.Now().String()
		if _, err = c.client.AppsV1().Deployments(config.KyvernoNamespace()).Update(context.TODO(), deploy, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "update Kyverno deployment")
		}
		return nil
	}
	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}
	exbackoff.Reset()
	return backoff.Retry(update, exbackoff)
}
