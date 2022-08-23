package tls

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// CertRenewalInterval is the renewal interval for rootCA
	CertRenewalInterval time.Duration = 12 * time.Hour
	// CAValidityDuration is the valid duration for CA certificates
	CAValidityDuration time.Duration = 365 * 24 * time.Hour
	// TLSValidityDuration is the valid duration for TLS certificates
	TLSValidityDuration time.Duration = 150 * 24 * time.Hour
	// ManagedByLabel is added to Kyverno managed secrets
	ManagedByLabel string = "cert.kyverno.io/managed-by"
	RootCAKey      string = "rootCA.crt"
)

// CertRenewer creates rootCA and pem pair to register
// webhook configurations and webhook server
// renews RootCA at the given interval
type CertRenewer struct {
	client              kubernetes.Interface
	certRenewalInterval time.Duration
	caValidityDuration  time.Duration
	tlsValidityDuration time.Duration
	certProps           *certificateProps

	// IP address where Kyverno controller runs. Only required if out-of-cluster.
	serverIP string
}

// NewCertRenewer returns an instance of CertRenewer
func NewCertRenewer(client kubernetes.Interface, clientConfig *rest.Config, certRenewalInterval, caValidityDuration, tlsValidityDuration time.Duration, serverIP string, log logr.Logger) (*CertRenewer, error) {
	certProps, err := newCertificateProps(clientConfig)
	if err != nil {
		return nil, err
	}
	return &CertRenewer{
		client:              client,
		certRenewalInterval: certRenewalInterval,
		caValidityDuration:  caValidityDuration,
		tlsValidityDuration: tlsValidityDuration,
		certProps:           certProps,
		serverIP:            serverIP,
	}, nil
}

// InitTLSPemPair Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
func (c *CertRenewer) InitTLSPemPair() error {
	if err := c.RenewCA(); err != nil {
		return err
	}
	if err := c.RenewTLS(); err != nil {
		return err
	}
	return nil
}

// RenewTLS renews the CA certificate if needed
func (c *CertRenewer) RenewCA() error {
	secret, key, certs, err := c.decodeCASecret()
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to read CA")
		return err
	}
	now := time.Now()
	certs = removeExpiredCertificates(now, certs...)
	if !allCertificatesExpired(now.Add(5*c.certRenewalInterval), certs...) {
		logger.V(4).Info("CA certificate does not need to be renewed")
		return nil
	}
	if !IsSecretManagedByKyverno(secret) {
		err := fmt.Errorf("tls is not valid but certificates are not managed by kyverno, we can't renew them")
		logger.Error(err, "tls is not valid but certificates are not managed by kyverno, we can't renew them")
		return err
	}
	caKey, caCert, err := generateCA(key, c.caValidityDuration)
	if err != nil {
		logger.Error(err, "failed to generate CA")
		return err
	}
	certs = append(certs, caCert)
	if err := c.writeCASecret(caKey, certs...); err != nil {
		logger.Error(err, "failed to write CA")
		return err
	}
	logger.Info("CA was renewed")
	return nil
}

// RenewTLS renews the TLS certificate if needed
func (c *CertRenewer) RenewTLS() error {
	_, caKey, caCerts, err := c.decodeCASecret()
	if err != nil {
		logger.Error(err, "failed to read CA")
		return err
	}
	secret, _, cert, err := c.decodeTLSSecret()
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to read TLS")
		return err
	}
	now := time.Now()
	if cert != nil && !allCertificatesExpired(now.Add(5*c.certRenewalInterval), cert) {
		logger.V(4).Info("TLS certificate does not need to be renewed")
		return nil
	}
	if !IsSecretManagedByKyverno(secret) {
		err := fmt.Errorf("tls is not valid but certificates are not managed by kyverno, we can't renew them")
		logger.Error(err, "tls is not valid but certificates are not managed by kyverno, we can't renew them")
		return err
	}
	tlsKey, tlsCert, err := generateTLS(c.certProps, c.serverIP, caCerts[len(caCerts)-1], caKey, c.tlsValidityDuration)
	if err != nil {
		logger.Error(err, "failed to generate TLS")
		return err
	}
	if err := c.writeTLSSecret(tlsKey, tlsCert); err != nil {
		logger.Error(err, "failed to write TLS")
		return err
	}
	logger.Info("TLS was renewed")
	return nil
}

// ValidateCert validates the CA Cert
func (c *CertRenewer) ValidateCert() (bool, error) {
	_, _, caCerts, err := c.decodeCASecret()
	if err != nil {
		return false, err
	}
	_, _, cert, err := c.decodeTLSSecret()
	if err != nil {
		return false, err
	}
	return validateCert(time.Now(), cert, caCerts...), nil
}

func (c *CertRenewer) getSecret(name string) (*corev1.Secret, error) {
	if s, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (c *CertRenewer) decodeSecret(name string) (*corev1.Secret, *rsa.PrivateKey, []*x509.Certificate, error) {
	secret, err := c.getSecret(name)
	if err != nil {
		return nil, nil, nil, err
	}
	var certBytes, keyBytes []byte
	if secret != nil {
		keyBytes = secret.Data[corev1.TLSPrivateKeyKey]
		certBytes = secret.Data[corev1.TLSCertKey]
		if len(certBytes) == 0 {
			certBytes = secret.Data[RootCAKey]
		}
	}
	var key *rsa.PrivateKey
	if keyBytes != nil {
		usedkey, err := pemToPrivateKey(keyBytes)
		if err != nil {
			return nil, nil, nil, err
		}
		key = usedkey
	}
	return secret, key, pemToCertificates(certBytes), nil
}

func (c *CertRenewer) decodeCASecret() (*corev1.Secret, *rsa.PrivateKey, []*x509.Certificate, error) {
	return c.decodeSecret(GenerateRootCASecretName())
}

func (c *CertRenewer) decodeTLSSecret() (*corev1.Secret, *rsa.PrivateKey, *x509.Certificate, error) {
	secret, key, certs, err := c.decodeSecret(GenerateTLSPairSecretName())
	if err != nil {
		return nil, nil, nil, err
	}
	if len(certs) == 0 {
		return secret, key, nil, nil
	} else if len(certs) == 1 {
		return secret, key, certs[0], nil
	} else {
		return nil, nil, nil, err
	}
}

func (c *CertRenewer) writeSecret(name string, key *rsa.PrivateKey, certs ...*x509.Certificate) error {
	logger := logger.WithValues("name", name, "namespace", config.KyvernoNamespace())
	secret, err := c.getSecret(name)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get CA secret")
		return err
	}
	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
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
		corev1.TLSCertKey:       certificateToPem(certs...),
		corev1.TLSPrivateKeyKey: privateKeyToPem(key),
	}
	if secret.ResourceVersion == "" {
		if _, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to update secret")
			return err
		} else {
			logger.Info("secret created")
		}
	} else {
		if _, err := c.client.CoreV1().Secrets(config.KyvernoNamespace()).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			logger.Error(err, "failed to update secret")
			return err
		} else {
			logger.Info("secret updated")
		}
	}
	return nil
}

// writeCASecret stores the CA cert in secret
func (c *CertRenewer) writeCASecret(key *rsa.PrivateKey, certs ...*x509.Certificate) error {
	return c.writeSecret(GenerateRootCASecretName(), key, certs...)
}

// writeTLSSecret Writes the pair of TLS certificate and key to the specified secret.
func (c *CertRenewer) writeTLSSecret(key *rsa.PrivateKey, cert *x509.Certificate) error {
	return c.writeSecret(GenerateTLSPairSecretName(), key, cert)
}
