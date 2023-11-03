package tls

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
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
	rootCAKey           = "rootCA.crt"
)

type CertValidator interface {
	// ValidateCert checks the certificates validity
	ValidateCert(context.Context) (bool, error)
}

type CertRenewer interface {
	// RenewCA renews the CA certificate if needed
	RenewCA(context.Context) error
	// RenewTLS renews the TLS certificate if needed
	RenewTLS(context.Context) error
}

type client interface {
	Get(context.Context, string, metav1.GetOptions) (*corev1.Secret, error)
	Create(context.Context, *corev1.Secret, metav1.CreateOptions) (*corev1.Secret, error)
	Update(context.Context, *corev1.Secret, metav1.UpdateOptions) (*corev1.Secret, error)
	Delete(context.Context, string, metav1.DeleteOptions) error
}

// certRenewer creates rootCA and pem pair to register
// webhook configurations and webhook server
// renews RootCA at the given interval
type certRenewer struct {
	client              client
	certRenewalInterval time.Duration
	caValidityDuration  time.Duration
	tlsValidityDuration time.Duration

	// server is an IP address or domain name where Kyverno controller runs. Only required if out-of-cluster.
	server     string
	commonName string
	dnsNames   []string
	namespace  string
	caSecret   string
	pairSecret string
}

// NewCertRenewer returns an instance of CertRenewer
func NewCertRenewer(
	client client,
	certRenewalInterval,
	caValidityDuration,
	tlsValidityDuration time.Duration,
	server string,
	commonName string,
	dnsNames []string,
	namespace string,
	caSecret string,
	pairSecret string,
) *certRenewer {
	return &certRenewer{
		client:              client,
		certRenewalInterval: certRenewalInterval,
		caValidityDuration:  caValidityDuration,
		tlsValidityDuration: tlsValidityDuration,
		server:              server,
		commonName:          commonName,
		dnsNames:            dnsNames,
		namespace:           namespace,
		caSecret:            caSecret,
		pairSecret:          pairSecret,
	}
}

// RenewCA renews the CA certificate if needed
func (c *certRenewer) RenewCA(ctx context.Context) error {
	secret, key, certs, err := c.decodeCASecret(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to read CA (%w)", err)
	}
	now := time.Now()
	certs = removeExpiredCertificates(now, certs...)
	if !allCertificatesExpired(now.Add(5*c.certRenewalInterval), certs...) {
		return nil
	}
	if !isSecretManagedByKyverno(secret) {
		return fmt.Errorf("tls is not valid but certificates are not managed by kyverno, we can't renew them")
	}
	if secret != nil && secret.Type != corev1.SecretTypeTLS {
		return c.client.Delete(ctx, secret.Name, metav1.DeleteOptions{})
	}
	caKey, caCert, err := generateCA(key, c.caValidityDuration)
	if err != nil {
		return fmt.Errorf("failed to generate CA (%w)", err)
	}
	certs = append(certs, caCert)
	if err := c.writeCASecret(ctx, caKey, certs...); err != nil {
		return fmt.Errorf("failed to write CA (%w)", err)
	}
	valid, err := c.ValidateCert(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate certs (%w)", err)
	}
	if !valid {
		if err := c.RenewTLS(ctx); err != nil {
			return fmt.Errorf("failed to renew TLS certificate (%w)", err)
		}
	}

	return nil
}

// RenewTLS renews the TLS certificate if needed
func (c *certRenewer) RenewTLS(ctx context.Context) error {
	_, caKey, caCerts, err := c.decodeCASecret(ctx)
	if err != nil {
		return fmt.Errorf("failed to read CA (%w)", err)
	}
	secret, _, cert, err := c.decodeTLSSecret(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to read TLS (%w)", err)
	}
	now := time.Now()
	if cert != nil {
		valid, err := c.ValidateCert(ctx)
		if err != nil || !valid {
		} else if !allCertificatesExpired(now.Add(5*c.certRenewalInterval), cert) {
			return nil
		}
	}

	if !isSecretManagedByKyverno(secret) {
		return fmt.Errorf("tls is not valid but certificates are not managed by kyverno, we can't renew them")
	}
	if secret != nil && secret.Type != corev1.SecretTypeTLS {
		return c.client.Delete(ctx, secret.Name, metav1.DeleteOptions{})
	}
	tlsKey, tlsCert, err := generateTLS(c.server, caCerts[len(caCerts)-1], caKey, c.tlsValidityDuration, c.commonName, c.dnsNames)
	if err != nil {
		return fmt.Errorf("failed to generate TLS (%w)", err)
	}
	if err := c.writeTLSSecret(ctx, tlsKey, tlsCert); err != nil {
		return fmt.Errorf("failed to write TLS (%w)", err)
	}
	return nil
}

// ValidateCert validates the CA Cert
func (c *certRenewer) ValidateCert(ctx context.Context) (bool, error) {
	_, _, caCerts, err := c.decodeCASecret(ctx)
	if err != nil {
		return false, err
	}
	_, _, cert, err := c.decodeTLSSecret(ctx)
	if err != nil {
		return false, err
	}
	return validateCert(time.Now(), cert, caCerts...), nil
}

func (c *certRenewer) getSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	if s, err := c.client.Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, err
	} else {
		return s, nil
	}
}

func (c *certRenewer) decodeSecret(ctx context.Context, name string) (*corev1.Secret, *rsa.PrivateKey, []*x509.Certificate, error) {
	secret, err := c.getSecret(ctx, name)
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

func (c *certRenewer) decodeCASecret(ctx context.Context) (*corev1.Secret, *rsa.PrivateKey, []*x509.Certificate, error) {
	return c.decodeSecret(ctx, c.caSecret)
}

func (c *certRenewer) decodeTLSSecret(ctx context.Context) (*corev1.Secret, *rsa.PrivateKey, *x509.Certificate, error) {
	secret, key, certs, err := c.decodeSecret(ctx, c.pairSecret)
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

func (c *certRenewer) writeSecret(ctx context.Context, name string, key *rsa.PrivateKey, certs ...*x509.Certificate) error {
	secret, err := c.getSecret(ctx, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get CA secret (%w)", err)
	}
	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: c.namespace,
				Labels: map[string]string{
					kyverno.LabelCertManagedBy: kyverno.ValueKyvernoApp,
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
		if _, err := c.client.Create(ctx, secret, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create secret (%w)", err)
		}
	} else {
		if _, err := c.client.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update secret (%w)", err)
		}
	}
	return nil
}

// writeCASecret stores the CA cert in secret
func (c *certRenewer) writeCASecret(ctx context.Context, key *rsa.PrivateKey, certs ...*x509.Certificate) error {
	return c.writeSecret(ctx, c.caSecret, key, certs...)
}

// writeTLSSecret Writes the pair of TLS certificate and key to the specified secret.
func (c *certRenewer) writeTLSSecret(ctx context.Context, key *rsa.PrivateKey, cert *x509.Certificate) error {
	return c.writeSecret(ctx, c.pairSecret, key, cert)
}
