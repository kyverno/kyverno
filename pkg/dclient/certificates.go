package client

import (
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/kyverno/kyverno/pkg/config"
	tls "github.com/kyverno/kyverno/pkg/tls"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

// InitTLSPemPair Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func (c *Client) InitTLSPemPair(configuration *rest.Config, serverIP string) (*tls.PemPair, error) {
	logger := c.log.WithName("InitTLSPemPair")
	certProps, err := c.GetTLSCertProps(configuration)
	if err != nil {
		return nil, err
	}

	tlsPair, err := c.ReadTLSPair(certProps)
	if err == nil {
		logger.Info("using existing TLS key/certificate pair")
		return tlsPair, nil
	}
	logger.V(3).Info("unable to find TLS pair", "reason", err.Error())

	logger.Info("building key/certificate pair for TLS")
	tlsPair, err = c.buildTLSPemPair(certProps, serverIP)
	if err != nil {
		return nil, err
	}
	if err = c.WriteTLSPairToSecret(certProps, tlsPair); err != nil {
		return nil, fmt.Errorf("unable to save TLS pair to the cluster: %v", err)
	}

	return tlsPair, nil
}

// buildTLSPemPair Issues TLS certificate for webhook server using self-signed CA cert
// Returns signed and approved TLS certificate in PEM format
func (c *Client) buildTLSPemPair(props tls.CertificateProps, serverIP string) (*tls.PemPair, error) {
	caCert, caPEM, err := tls.GenerateCACert()
	if err != nil {
		return nil, err
	}

	if err := c.WriteCACertToSecret(caPEM, props); err != nil {
		return nil, fmt.Errorf("failed to write CA cert to secret: %v", err)
	}

	return tls.GenerateCertPem(caCert, props, serverIP)
}

//ReadRootCASecret returns the RootCA from the pre-defined secret
func (c *Client) ReadRootCASecret() (result []byte) {
	logger := c.log.WithName("ReadRootCASecret")
	certProps, err := c.GetTLSCertProps(c.clientConfig)
	if err != nil {
		logger.Error(err, "failed to get TLS Cert Properties")
		return result
	}
	sname := generateRootCASecretName(certProps)
	stlsca, err := c.GetResource("", Secrets, certProps.Namespace, sname)
	if err != nil {
		return result
	}
	tlsca, err := convertToSecret(stlsca)
	if err != nil {
		logger.Error(err, "failed to convert secret", "name", sname, "namespace", certProps.Namespace)
		return result
	}

	result = tlsca.Data[rootCAKey]
	if len(result) == 0 {
		logger.Info("root CA certificate not found in secret", "name", tlsca.Name, "namespace", certProps.Namespace)
		return result
	}
	logger.V(4).Info("using CA bundle defined in secret to validate the webhook's server certificate", "name", tlsca.Name, "namespace", certProps.Namespace)
	return result
}

const (
	// ManagedByLabel is added to Kyverno managed secrets
	ManagedByLabel string = "cert.kyverno.io/managed-by"

	selfSignedAnnotation string = "self-signed-cert"
	rootCAKey            string = "rootCA.crt"
)

// ReadTLSPair Reads the pair of TLS certificate and key from the specified secret.
func (c *Client) ReadTLSPair(props tls.CertificateProps) (*tls.PemPair, error) {
	sname := generateTLSPairSecretName(props)
	unstrSecret, err := c.GetResource("", Secrets, props.Namespace, sname)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %v", props.Namespace, sname, err)
	}

	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook configuration, check if the corresponding secret is created
	annotations := unstrSecret.GetAnnotations()
	if _, ok := annotations[selfSignedAnnotation]; ok {
		sname := generateRootCASecretName(props)
		_, err := c.GetResource("", Secrets, props.Namespace, sname)
		if err != nil {
			return nil, fmt.Errorf("root CA secret is required while using self-signed certificates TLS pair, defaulting to generating new TLS pair  %s/%s", props.Namespace, sname)
		}
	}
	secret, err := convertToSecret(unstrSecret)
	if err != nil {
		return nil, err
	}

	pemPair := tls.PemPair{
		Certificate: secret.Data[v1.TLSCertKey],
		PrivateKey:  secret.Data[v1.TLSPrivateKeyKey],
	}

	if len(pemPair.Certificate) == 0 {
		return nil, fmt.Errorf("TLS Certificate not found in secret %s/%s", props.Namespace, sname)
	}
	if len(pemPair.PrivateKey) == 0 {
		return nil, fmt.Errorf("TLS PrivateKey not found in secret %s/%s", props.Namespace, sname)
	}

	return &pemPair, nil
}

// WriteCACertToSecret stores the CA cert in secret
func (c *Client) WriteCACertToSecret(caPEM *tls.PemPair, props tls.CertificateProps) error {
	logger := c.log.WithName("CAcert")
	name := generateRootCASecretName(props)

	secretUnstr, err := c.GetResource("", Secrets, props.Namespace, name)
	if err != nil {
		secret := &v1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: props.Namespace,
				Annotations: map[string]string{
					selfSignedAnnotation: "true",
				},
				Labels: map[string]string{
					ManagedByLabel: "kyverno",
				},
			},
			Data: map[string][]byte{
				rootCAKey: caPEM.Certificate,
			},
			Type: v1.SecretTypeOpaque,
		}

		_, err := c.CreateResource("", Secrets, props.Namespace, secret, false)
		if err == nil {
			logger.Info("secret created", "name", name, "namespace", props.Namespace)
		}
		return err
	}

	if _, ok := secretUnstr.GetAnnotations()[selfSignedAnnotation]; !ok {
		secretUnstr.SetAnnotations(map[string]string{selfSignedAnnotation: "true"})
	}

	dataMap := map[string]interface{}{
		rootCAKey: base64.StdEncoding.EncodeToString(caPEM.Certificate)}

	if err := unstructured.SetNestedMap(secretUnstr.Object, dataMap, "data"); err != nil {
		return err
	}

	_, err = c.UpdateResource("", Secrets, props.Namespace, secretUnstr, false)
	if err != nil {
		return err
	}
	logger.Info("secret updated", "name", name, "namespace", props.Namespace)
	return nil
}

// WriteTLSPairToSecret Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (c *Client) WriteTLSPairToSecret(props tls.CertificateProps, pemPair *tls.PemPair) error {
	logger := c.log.WithName("WriteTLSPair")
	name := generateTLSPairSecretName(props)
	secretUnstr, err := c.GetResource("", Secrets, props.Namespace, name)
	if err != nil {
		secret := &v1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: props.Namespace,
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

		_, err := c.CreateResource("", Secrets, props.Namespace, secret, false)
		if err == nil {
			logger.Info("secret created", "name", name, "namespace", props.Namespace)
		}
		return err
	}

	dataMap := map[string]interface{}{
		v1.TLSCertKey:       base64.StdEncoding.EncodeToString(pemPair.Certificate),
		v1.TLSPrivateKeyKey: base64.StdEncoding.EncodeToString(pemPair.PrivateKey),
	}

	if err := unstructured.SetNestedMap(secretUnstr.Object, dataMap, "data"); err != nil {
		return err
	}

	_, err = c.UpdateResource("", Secrets, props.Namespace, secretUnstr, false)
	if err != nil {
		return err
	}

	logger.Info("secret updated", "name", name, "namespace", props.Namespace)
	return nil
}

func generateTLSPairSecretName(props tls.CertificateProps) string {
	return tls.GenerateInClusterServiceName(props) + ".kyverno-tls-pair"
}

func generateRootCASecretName(props tls.CertificateProps) string {
	return tls.GenerateInClusterServiceName(props) + ".kyverno-tls-ca"
}

//GetTLSCertProps provides the TLS Certificate Properties
func (c *Client) GetTLSCertProps(configuration *rest.Config) (certProps tls.CertificateProps, err error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return certProps, err
	}
	certProps = tls.CertificateProps{
		Service:       config.KyvernoServiceName,
		Namespace:     config.KyvernoNamespace,
		APIServerHost: apiServerURL.Hostname(),
	}
	return certProps, nil
}
