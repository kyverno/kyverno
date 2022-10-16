package tls

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CertRenewalInterval is the renewal interval for rootCA
	CertRenewalInterval = 12 * time.Hour
	// CAValidityDuration is the valid duration for CA certificates
	CAValidityDuration = 365 * 24 * time.Hour
	// TLSValidityDuration is the valid duration for TLS certificates
	TLSValidityDuration = 150 * 24 * time.Hour
	// managedByLabel is added to Kyverno managed secrets
	managedByLabel = "cert.kyverno.io/managed-by"
	rootCAKey      = "rootCA.crt"
)

type CertValidator interface {
	// ValidateCert checks the certificates validity
	ValidateCert() (bool, error)
}

type CertRenewer interface {
	// RenewCA renews the CA certificate if needed
	RenewCA() error
	// RenewTLS renews the TLS certificate if needed
	RenewTLS() error
}

// certRenewer creates rootCA and pem pair to register
// webhook configurations and webhook server
// renews RootCA at the given interval
type certRenewer struct {
	client              controllerutils.ObjectClient[*corev1.Secret]
	certRenewalInterval time.Duration
	caValidityDuration  time.Duration
	tlsValidityDuration time.Duration

	// server is an IP address or domain name where Kyverno controller runs. Only required if out-of-cluster.
	server string
}

// NewCertRenewer returns an instance of CertRenewer
func NewCertRenewer(client controllerutils.ObjectClient[*corev1.Secret], certRenewalInterval, caValidityDuration, tlsValidityDuration time.Duration, server string) *certRenewer {
	return &certRenewer{
		client:              client,
		certRenewalInterval: certRenewalInterval,
		caValidityDuration:  caValidityDuration,
		tlsValidityDuration: tlsValidityDuration,
		server:              server,
	}
}

// RenewCA renews the CA certificate if needed
func (c *certRenewer) RenewCA() error {
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
	if !isSecretManagedByKyverno(secret) {
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
func (c *certRenewer) RenewTLS() error {
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
	if !isSecretManagedByKyverno(secret) {
		err := fmt.Errorf("tls is not valid but certificates are not managed by kyverno, we can't renew them")
		logger.Error(err, "tls is not valid but certificates are not managed by kyverno, we can't renew them")
		return err
	}
	tlsKey, tlsCert, err := generateTLS(c.server, caCerts[len(caCerts)-1], caKey, c.tlsValidityDuration)
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
func (c *certRenewer) ValidateCert() (bool, error) {
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

func (c *certRenewer) getSecret(name string) (*corev1.Secret, error) {
	if s, err := c.client.Get(context.TODO(), name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (c *certRenewer) decodeSecret(name string) (*corev1.Secret, *rsa.PrivateKey, []*x509.Certificate, error) {
	secret, err := c.getSecret(name)
	if err != nil {
		return nil, nil, nil, err
	}
	var certBytes, keyBytes []byte
	if secret != nil {
		keyBytes = secret.Data[corev1.TLSPrivateKeyKey]
		certBytes = secret.Data[corev1.TLSCertKey]
		if len(certBytes) == 0 {
			certBytes = secret.Data[rootCAKey]
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

func (c *certRenewer) decodeCASecret() (*corev1.Secret, *rsa.PrivateKey, []*x509.Certificate, error) {
	return c.decodeSecret(GenerateRootCASecretName())
}

func (c *certRenewer) decodeTLSSecret() (*corev1.Secret, *rsa.PrivateKey, *x509.Certificate, error) {
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

func (c *certRenewer) writeSecret(name string, key *rsa.PrivateKey, certs ...*x509.Certificate) error {
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
					managedByLabel: kyvernov1.ValueKyvernoApp,
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
		if _, err := c.client.Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
			logger.Error(err, "failed to update secret")
			return err
		} else {
			logger.Info("secret created")
		}
	} else {
		if _, err := c.client.Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			logger.Error(err, "failed to update secret")
			return err
		} else {
			logger.Info("secret updated")
		}
	}
	return nil
}

// writeCASecret stores the CA cert in secret
func (c *certRenewer) writeCASecret(key *rsa.PrivateKey, certs ...*x509.Certificate) error {
	return c.writeSecret(GenerateRootCASecretName(), key, certs...)
}

// writeTLSSecret Writes the pair of TLS certificate and key to the specified secret.
func (c *certRenewer) writeTLSSecret(key *rsa.PrivateKey, cert *x509.Certificate) error {
	return c.writeSecret(GenerateTLSPairSecretName(), key, cert)
}
