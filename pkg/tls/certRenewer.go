package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

const (
	// ManagedByLabel is added to Kyverno managed secrets
	ManagedByLabel      string = "cert.kyverno.io/managed-by"
	MasterDeploymentUID string = "cert.kyverno.io/master-deployment-uid"

	SelfSignedAnnotation    string = "self-signed-cert"
	RootCAKey               string = "rootCA.crt"
	rollingUpdateAnnotation string = "update.kyverno.io/force-rolling-update"
)

// CertRenewer creates rootCA and pem pair to register
// webhook configurations and webhook server
// renews RootCA at the given interval
type CertRenewer struct {
	client               *client.Client
	clientConfig         *rest.Config
	certRenewalInterval  time.Duration
	certValidityDuration time.Duration

	// IP address where Kyverno controller runs. Only required if out-of-cluster.
	serverIP string

	log logr.Logger
}

// NewCertRenewer returns an instance of CertRenewer
func NewCertRenewer(client *client.Client, clientConfig *rest.Config, certRenewalInterval, certValidityDuration time.Duration, serverIP string, log logr.Logger) *CertRenewer {
	return &CertRenewer{
		client:               client,
		clientConfig:         clientConfig,
		certRenewalInterval:  certRenewalInterval,
		certValidityDuration: certValidityDuration,
		serverIP:             serverIP,
		log:                  log,
	}
}

func (c *CertRenewer) Client() *client.Client {
	return c.client
}

func (c *CertRenewer) ClientConfig() *rest.Config {
	return c.clientConfig
}

// InitTLSPemPair Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func (c *CertRenewer) InitTLSPemPair() (*PemPair, error) {
	logger := c.log.WithName("InitTLSPemPair")
	certProps, err := GetTLSCertProps(c.clientConfig)
	if err != nil {
		return nil, err
	}

	if valid, err := c.ValidCert(); err == nil && valid {
		if tlsPair, err := ReadTLSPair(c.clientConfig, c.client); err == nil {
			logger.Info("using existing TLS key/certificate pair")
			return tlsPair, nil
		}
	} else if err != nil {
		logger.V(3).Info("unable to find TLS pair", "reason", err.Error())
	}

	logger.Info("building key/certificate pair for TLS")
	return c.buildTLSPemPairAndWriteToSecrets(certProps, c.serverIP)
}

// buildTLSPemPairAndWriteToSecrets Issues TLS certificate for webhook server using self-signed CA cert
// Returns signed and approved TLS certificate in PEM format
func (c *CertRenewer) buildTLSPemPairAndWriteToSecrets(props CertificateProps, serverIP string) (*PemPair, error) {
	caCert, caPEM, err := GenerateCACert(c.certValidityDuration)
	if err != nil {
		return nil, err
	}

	if err := c.WriteCACertToSecret(caPEM, props); err != nil {
		return nil, fmt.Errorf("failed to write CA cert to secret: %v", err)
	}

	tlsPair, err := GenerateCertPem(caCert, props, serverIP, c.certValidityDuration)
	if err != nil {
		return nil, err
	}

	if err = c.WriteTLSPairToSecret(props, tlsPair); err != nil {
		return nil, fmt.Errorf("unable to save TLS pair to the cluster: %v", err)
	}

	return tlsPair, nil
}

// ReadTLSPair Reads the pair of TLS certificate and key from the specified secret.

// WriteCACertToSecret stores the CA cert in secret
func (c *CertRenewer) WriteCACertToSecret(caPEM *PemPair, props CertificateProps) error {
	logger := c.log.WithName("CAcert")
	name := GenerateRootCASecretName(props)

	depl, err := c.client.GetResource("", "Deployment", props.Namespace, config.KyvernoDeploymentName)

	deplHash := ""
	if err == nil {
		deplHash = fmt.Sprintf("%v", depl.GetUID())
	}

	secretUnstr, err := c.client.GetResource("", "Secret", props.Namespace, name)

	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: props.Namespace,
			Annotations: map[string]string{
				SelfSignedAnnotation: "true",
				MasterDeploymentUID:  deplHash,
			},
			Labels: map[string]string{
				ManagedByLabel: "kyverno",
			},
		},
		Data: map[string][]byte{
			RootCAKey: caPEM.Certificate,
		},
		Type: v1.SecretTypeOpaque,
	}

	if err != nil {
		if k8errors.IsNotFound(err) {
			_, err = c.client.CreateResource("", "Secret", props.Namespace, secret, false)
			if err == nil {
				logger.Info("secret created", "name", name, "namespace", props.Namespace)
			}
		}
		return err
	} else if CanAddAnnotationToSecret(deplHash, secretUnstr) {
		_, err = c.client.UpdateResource("", "Secret", props.Namespace, secret, false)
		if err == nil {
			logger.Info("secret updated", "name", name, "namespace", props.Namespace)
		}
		return err
	}

	if _, ok := secretUnstr.GetAnnotations()[SelfSignedAnnotation]; !ok {
		secretUnstr.SetAnnotations(map[string]string{SelfSignedAnnotation: "true"})
	}

	dataMap := map[string]interface{}{
		RootCAKey: base64.StdEncoding.EncodeToString(caPEM.Certificate)}

	if err = unstructured.SetNestedMap(secretUnstr.Object, dataMap, "data"); err != nil {
		return err
	}

	_, err = c.client.UpdateResource("", "Secret", props.Namespace, secretUnstr, false)
	if err != nil {
		return err
	}
	logger.Info("secret updated", "name", name, "namespace", props.Namespace)
	return nil
}

// WriteTLSPairToSecret Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (c *CertRenewer) WriteTLSPairToSecret(props CertificateProps, pemPair *PemPair) error {
	logger := c.log.WithName("WriteTLSPair")

	name := GenerateTLSPairSecretName(props)

	depl, err := c.client.GetResource("", "Deployment", props.Namespace, config.KyvernoDeploymentName)

	deplHash := ""
	if err == nil {
		deplHash = fmt.Sprintf("%v", depl.GetUID())
	}

	secretUnstr, err := c.client.GetResource("", "Secret", props.Namespace, name)

	secretPtr := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: props.Namespace,
			Annotations: map[string]string{
				MasterDeploymentUID: deplHash,
			},
			Labels: map[string]string{
				ManagedByLabel: "kyverno",
			},
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       pemPair.Certificate,
			v1.TLSPrivateKeyKey: pemPair.PrivateKey,
		},
		Type: v1.SecretTypeTLS,
	}

	if err != nil {
		if k8errors.IsNotFound(err) {
			_, err = c.client.CreateResource("", "Secret", props.Namespace, secretPtr, false)
			if err == nil {
				logger.Info("secret created", "name", name, "namespace", props.Namespace)
			}
		}
		return err
	} else if CanAddAnnotationToSecret(deplHash, secretUnstr) {
		_, err = c.client.UpdateResource("", "Secret", props.Namespace, secretPtr, false)
		if err == nil {
			logger.Info("secret updated", "name", name, "namespace", props.Namespace)
		}
		return err
	}

	dataMap := map[string][]byte{
		v1.TLSCertKey:       pemPair.Certificate,
		v1.TLSPrivateKeyKey: pemPair.PrivateKey,
	}

	secret, err := convertToSecret(secretUnstr)
	if err != nil {
		return err
	}

	secret.Data = dataMap
	_, err = c.client.UpdateResource("", "Secret", props.Namespace, secret, false)
	if err != nil {
		return err
	}

	logger.Info("secret updated", "name", name, "namespace", props.Namespace)
	return nil
}

// RollingUpdate triggers a rolling update of Kyverno pod.
// It is used when the rootCA is renewed, the restart of
// Kyverno pod will register webhook server with new cert
func (c *CertRenewer) RollingUpdate() error {

	update := func() error {
		deploy, err := c.client.GetResource("", "Deployment", config.KyvernoNamespace, config.KyvernoDeploymentName)
		if err != nil {
			return errors.Wrap(err, "failed to find Kyverno")
		}

		if IsKyvernoInRollingUpdate(deploy.UnstructuredContent(), c.log) {
			return nil
		}

		annotations, ok, err := unstructured.NestedStringMap(deploy.UnstructuredContent(), "spec", "template", "metadata", "annotations")
		if err != nil {
			return errors.Wrap(err, "bad annotations")
		}

		if !ok {
			annotations = map[string]string{}
		}

		annotations[rollingUpdateAnnotation] = time.Now().String()
		if err = unstructured.SetNestedStringMap(deploy.UnstructuredContent(),
			annotations,
			"spec", "template", "metadata", "annotations",
		); err != nil {
			return errors.Wrapf(err, "set annotation %s", rollingUpdateAnnotation)
		}

		if _, err = c.client.UpdateResource("", "Deployment", config.KyvernoNamespace, deploy, false); err != nil {
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

// ValidCert validates the CA Cert
func (c *CertRenewer) ValidCert() (bool, error) {
	logger := c.log.WithName("ValidCert")

	certProps, err := GetTLSCertProps(c.clientConfig)
	if err != nil {
		return false, nil
	}
	var managedByKyverno bool
	snameTLS := GenerateTLSPairSecretName(certProps)
	snameCA := GenerateRootCASecretName(certProps)
	unstrSecret, err := c.client.GetResource("", "Secret", certProps.Namespace, snameTLS)
	if err != nil {
		return false, nil
	}

	if label, ok := unstrSecret.GetLabels()[ManagedByLabel]; ok {
		managedByKyverno = label == "kyverno"
	}

	_, ok := unstrSecret.GetAnnotations()[MasterDeploymentUID]
	if managedByKyverno && !ok {
		return false, nil
	}

	unstrSecret, err = c.client.GetResource("", "Secret", certProps.Namespace, snameCA)
	if err != nil {
		return false, nil
	}

	if label, ok := unstrSecret.GetLabels()[ManagedByLabel]; ok {
		managedByKyverno = label == "kyverno"
	}

	_, ok = unstrSecret.GetAnnotations()[MasterDeploymentUID]
	if managedByKyverno && !ok {
		return false, nil
	}

	rootCA, err := ReadRootCASecret(c.clientConfig, c.client)
	if err != nil {
		return false, errors.Wrap(err, "unable to read CA from secret")
	}

	tlsPair, err := ReadTLSPair(c.clientConfig, c.client)
	if err != nil {
		// wait till next reconcile
		logger.Info("unable to read TLS PEM Pair from secret", "reason", err.Error())
		return false, errors.Wrap(err, "unable to read TLS PEM Pair from secret")
	}

	// build cert pool
	pool := x509.NewCertPool()
	caPem, _ := pem.Decode(rootCA)
	if caPem == nil {
		logger.Error(err, "bad certificate")
		return false, nil
	}

	cac, err := x509.ParseCertificate(caPem.Bytes)
	if err != nil {
		logger.Error(err, "failed to parse CA cert")
		return false, nil
	}
	pool.AddCert(cac)

	// valid PEM pair
	_, err = tls.X509KeyPair(tlsPair.Certificate, tlsPair.PrivateKey)
	if err != nil {
		logger.Error(err, "invalid PEM pair")
		return false, nil
	}

	certPem, _ := pem.Decode(tlsPair.Certificate)
	if certPem == nil {
		logger.Error(err, "bad private key")
		return false, nil
	}

	cert, err := x509.ParseCertificate(certPem.Bytes)
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

// IsKyvernoInRollingUpdate returns true if Kyverno is in rolling update
func IsKyvernoInRollingUpdate(deploy map[string]interface{}, logger logr.Logger) bool {
	replicas, _, err := unstructured.NestedFloat64(deploy, "spec", "replicas")
	if err != nil {
		logger.Error(err, "unable to fetch spec.replicas")
	}

	nonTerminatedReplicas, _, err := unstructured.NestedFloat64(deploy, "status", "replicas")
	if err != nil {
		logger.Error(err, "unable to fetch status.replicas")
	}

	if nonTerminatedReplicas > replicas {
		logger.Info("detect Kyverno is in rolling update, won't trigger the update again")
		return true
	}

	return false
}

func GenerateTLSPairSecretName(props CertificateProps) string {
	return generateInClusterServiceName(props) + ".kyverno-tls-pair"
}

func GenerateRootCASecretName(props CertificateProps) string {
	return generateInClusterServiceName(props) + ".kyverno-tls-ca"
}

func CanAddAnnotationToSecret(deplHash string, secret *unstructured.Unstructured) bool {
	var deplHashSec string = "default"
	var ok, managedByKyverno bool

	if label, ok := secret.GetLabels()[ManagedByLabel]; ok {
		managedByKyverno = label == "kyverno"
	}
	deplHashSec, ok = secret.GetAnnotations()[MasterDeploymentUID]

	return managedByKyverno && (!ok || deplHashSec != deplHash)
}
