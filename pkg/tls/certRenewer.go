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
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
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
	rootCAKey               string = "rootCA.crt"
	rollingUpdateAnnotation string = "update.kyverno.io/force-rolling-update"
)

// CertRenewer creates rootCA and pem pair to register
// webhook configurations and webhook server
// renews RootCA at the given interval
type CertRenewer struct {
	client               kubernetes.Interface
	certRenewalInterval  time.Duration
	certValidityDuration time.Duration
	certProps            *CertificateProps

	// IP address where Kyverno controller runs. Only required if out-of-cluster.
	serverIP string

	log logr.Logger
}

// NewCertRenewer returns an instance of CertRenewer
func NewCertRenewer(client kubernetes.Interface, clientConfig *rest.Config, certRenewalInterval, certValidityDuration time.Duration, serverIP string, log logr.Logger) (*CertRenewer, error) {
	certProps, err := NewCertificateProps(clientConfig)
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
// Returns struct with key/certificate pair.
func (c *CertRenewer) InitTLSPemPair() error {
	logger := c.log.WithName("InitTLSPemPair")
	if valid, err := c.ValidCert(); err == nil && valid {
		if _, _, err := ReadTLSPair(c.client); err == nil {
			logger.Info("using existing TLS key/certificate pair")
			return nil
		}
	} else if err != nil {
		logger.V(3).Info("unable to find TLS pair", "reason", err.Error())
	}

	logger.Info("building key/certificate pair for TLS")
	return c.buildTLSPemPairAndWriteToSecrets(c.serverIP)
}

// buildTLSPemPairAndWriteToSecrets Issues TLS certificate for webhook server using self-signed CA cert
// Returns signed and approved TLS certificate in PEM format
func (c *CertRenewer) buildTLSPemPairAndWriteToSecrets(serverIP string) error {
	caCert, err := GenerateCA(c.certValidityDuration)
	if err != nil {
		return err
	}
	tlsPair, err := GenerateCert(caCert, c.certProps, serverIP, c.certValidityDuration)
	if err != nil {
		return err
	}
	if err := c.writeCACertToSecret(caCert); err != nil {
		return fmt.Errorf("failed to write CA cert to secret: %v", err)
	}
	if err = c.writeTLSPairToSecret(tlsPair); err != nil {
		return fmt.Errorf("unable to save TLS pair to the cluster: %v", err)
	}
	return nil
}

// writeCACertToSecret stores the CA cert in secret
func (c *CertRenewer) writeCACertToSecret(ca *KeyPair) error {
	logger := c.log.WithName("CAcert")
	name := GenerateRootCASecretName()
	caBytes := CertificateToPem(ca.Cert)
	keyBytes := PrivateKeyToPem(ca.Key)
	secret, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8errors.IsNotFound(err) {
			secret = &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: config.KyvernoNamespace(),
					Labels: map[string]string{
						ManagedByLabel: "kyverno",
					},
				},
				Data: map[string][]byte{
					v1.TLSCertKey:       caBytes,
					v1.TLSPrivateKeyKey: keyBytes,
				},
				Type: v1.SecretTypeTLS,
			}
			_, err = c.client.CoreV1().Secrets(config.KyvernoNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err == nil {
				logger.Info("secret created", "name", name, "namespace", config.KyvernoNamespace())
			}
		}
		return err
	}
	if !IsSecretManagedByKyverno(secret) {
		return fmt.Errorf("secret %s is not managed by kyverno and cannot be updated", secret.GetName())
	}
	secret.Type = v1.SecretTypeTLS
	secret.Data = map[string][]byte{
		v1.TLSCertKey:       caBytes,
		v1.TLSPrivateKeyKey: keyBytes,
	}
	_, err = c.client.CoreV1().Secrets(config.KyvernoNamespace()).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	logger.Info("secret updated", "name", name, "namespace", config.KyvernoNamespace())
	return nil
}

// writeTLSPairToSecret Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (c *CertRenewer) writeTLSPairToSecret(tls *KeyPair) error {
	logger := c.log.WithName("WriteTLSPair")
	name := GenerateTLSPairSecretName()
	certBytes := CertificateToPem(tls.Cert)
	keyBytes := PrivateKeyToPem(tls.Key)
	secret, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if k8errors.IsNotFound(err) {
			secret = &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: config.KyvernoNamespace(),
					Labels: map[string]string{
						ManagedByLabel: "kyverno",
					},
				},
				Data: map[string][]byte{
					v1.TLSCertKey:       certBytes,
					v1.TLSPrivateKeyKey: keyBytes,
				},
				Type: v1.SecretTypeTLS,
			}
			_, err = c.client.CoreV1().Secrets(config.KyvernoNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err == nil {
				logger.Info("secret created", "name", name, "namespace", config.KyvernoNamespace())
			}
		}
		return err
	}
	if !IsSecretManagedByKyverno(secret) {
		return fmt.Errorf("secret %s is not managed by kyverno and cannot be updated", secret.GetName())
	}
	secret.Data = map[string][]byte{
		v1.TLSCertKey:       certBytes,
		v1.TLSPrivateKeyKey: keyBytes,
	}
	_, err = c.client.CoreV1().Secrets(config.KyvernoNamespace()).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	logger.Info("secret updated", "name", name, "namespace", config.KyvernoNamespace())
	return nil
}

// ValidCert validates the CA Cert
func (c *CertRenewer) ValidCert() (bool, error) {
	logger := c.log.WithName("ValidCert")
	rootCA, err := ReadRootCASecret(c.client)
	if err != nil {
		return false, errors.Wrap(err, "unable to read CA from secret")
	}
	certPem, keyPem, err := ReadTLSPair(c.client)
	if err != nil {
		// wait till next reconcile
		logger.Info("unable to read TLS PEM Pair from secret", "reason", err.Error())
		return false, errors.Wrap(err, "unable to read TLS PEM Pair from secret")
	}
	// build cert pool
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootCA) {
		logger.Error(err, "bad certificate")
		return false, nil
	} // valid PEM pair
	_, err = tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		logger.Error(err, "invalid PEM pair")
		return false, nil
	}
	certPemBlock, _ := pem.Decode(certPem)
	if certPem == nil {
		logger.Error(err, "bad private key")
		return false, nil
	}
	cert, err := x509.ParseCertificate(certPemBlock.Bytes)
	if err != nil {
		logger.Error(err, "failed to parse cert")
		return false, nil
	}
	if _, err = cert.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: time.Now().Add(c.certRenewalInterval),
	}); err != nil {
		logger.Error(err, "invalid cert")
		return false, nil
	}
	return true, nil
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
