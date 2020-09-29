package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/nirmata/kyverno/pkg/config"
	tls "github.com/nirmata/kyverno/pkg/tls"
	certificates "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// InitTLSPemPair Loads or creates PEM private key and TLS certificate for webhook server.
// Created pair is stored in cluster's secret.
// Returns struct with key/certificate pair.
func (c *Client) InitTLSPemPair(ctx context.Context, configuration *rest.Config, fqdncn bool) (*tls.TlsPemPair, error) {
	logger := c.log
	certProps, err := c.GetTLSCertProps(configuration)
	if err != nil {
		return nil, err
	}
	tlsPair := c.ReadTlsPair(ctx, certProps)
	if tls.IsTLSPairShouldBeUpdated(tlsPair) {
		logger.Info("Generating new key/certificate pair for TLS")
		tlsPair, err = c.generateTLSPemPair(ctx, certProps, fqdncn)
		if err != nil {
			return nil, err
		}
		if err = c.WriteTlsPair(ctx, certProps, tlsPair); err != nil {
			return nil, fmt.Errorf("Unable to save TLS pair to the cluster: %v", err)
		}
		return tlsPair, nil
	}
	logger.Info("Using existing TLS key/certificate pair")
	return tlsPair, nil
}

//generateTlsPemPair Issues TLS certificate for webhook server using given PEM private key
// Returns signed and approved TLS certificate in PEM format
func (c *Client) generateTLSPemPair(ctx context.Context, props tls.TlsCertificateProps, fqdncn bool) (*tls.TlsPemPair, error) {
	privateKey, err := tls.TLSGeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	certRequest, err := tls.CertificateGenerateRequest(privateKey, props, fqdncn)
	if err != nil {
		return nil, fmt.Errorf("Unable to create certificate request: %v", err)
	}

	certRequest, err = c.submitAndApproveCertificateRequest(ctx, certRequest)
	if err != nil {
		return nil, fmt.Errorf("Unable to submit and approve certificate request: %v", err)
	}

	tlsCert, err := c.fetchCertificateFromRequest(ctx, certRequest, 10)
	if err != nil {
		return nil, fmt.Errorf("Failed to configure a certificate for the Kyverno controller. A CA certificate is required to allow the Kubernetes API Server to communicate with Kyverno. You can either provide a certificate or configure your cluster to allow certificate signing. Please refer to https://github.com/nirmata/kyverno/installation.md.: %v", err)
	}

	return &tls.TlsPemPair{
		Certificate: tlsCert,
		PrivateKey:  tls.TLSPrivateKeyToPem(privateKey),
	}, nil
}

// Submits and approves certificate request, returns request which need to be fetched
func (c *Client) submitAndApproveCertificateRequest(ctx context.Context, req *certificates.CertificateSigningRequest) (*certificates.CertificateSigningRequest, error) {
	logger := c.log.WithName("submitAndApproveCertificateRequest")
	certClient, err := c.GetCSRInterface()
	if err != nil {
		return nil, err
	}
	csrList, err := c.ListResource(ctx, "", CSRs, "", nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to list existing certificate requests: %v", err)
	}

	for _, csr := range csrList.Items {
		if csr.GetName() == req.ObjectMeta.Name {
			err := c.DeleteResource(ctx, "", CSRs, "", csr.GetName(), false)
			if err != nil {
				return nil, fmt.Errorf("Unable to delete existing certificate request: %v", err)
			}
			logger.Info("Old certificate request is deleted")
			break
		}
	}

	unstrRes, err := c.CreateResource(ctx, "", CSRs, "", req, false)
	if err != nil {
		return nil, err
	}
	logger.Info("Certificate request created", "name", unstrRes.GetName())

	res, err := convertToCSR(unstrRes)
	if err != nil {
		return nil, err
	}
	res.Status.Conditions = append(res.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "NKP-Approve",
		Message: "This CSR was approved by Nirmata kyverno controller",
	})
	res, err = certClient.UpdateApproval(ctx, res, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("Unable to approve certificate request: %v", err)
	}
	logger.Info("Certificate request is approved", "name", res.ObjectMeta.Name)

	return res, nil
}

// Fetches certificate from given request. Tries to obtain certificate for maxWaitSeconds
func (c *Client) fetchCertificateFromRequest(ctx context.Context, req *certificates.CertificateSigningRequest, maxWaitSeconds uint8) ([]byte, error) {
	// TODO: react of SIGINT and SIGTERM
	timeStart := time.Now()
	for time.Since(timeStart) < time.Duration(maxWaitSeconds)*time.Second {
		unstrR, err := c.GetResource(ctx, "", CSRs, "", req.ObjectMeta.Name)
		if err != nil {
			return nil, err
		}
		r, err := convertToCSR(unstrR)
		if err != nil {
			return nil, err
		}

		if r.Status.Certificate != nil {
			return r.Status.Certificate, nil
		}

		for _, condition := range r.Status.Conditions {
			if condition.Type == certificates.CertificateDenied {
				return nil, errors.New(condition.String())
			}
		}
	}
	return nil, fmt.Errorf("Cerificate fetch timeout is reached: %d seconds", maxWaitSeconds)
}

//ReadRootCASecret returns the RootCA from the pre-defined secret
func (c *Client) ReadRootCASecret(ctx context.Context) (result []byte) {
	logger := c.log.WithName("ReadRootCASecret")
	certProps, err := c.GetTLSCertProps(c.clientConfig)
	if err != nil {
		logger.Error(err, "failed to get TLS Cert Properties")
		return result
	}
	sname := generateRootCASecretName(certProps)
	stlsca, err := c.GetResource(ctx, "", Secrets, certProps.Namespace, sname)
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

const selfSignedAnnotation string = "self-signed-cert"
const rootCAKey string = "rootCA.crt"

//ReadTlsPair Reads the pair of TLS certificate and key from the specified secret.
func (c *Client) ReadTlsPair(ctx context.Context, props tls.TlsCertificateProps) *tls.TlsPemPair {
	logger := c.log.WithName("ReadTlsPair")
	sname := generateTLSPairSecretName(props)
	unstrSecret, err := c.GetResource(ctx, "", Secrets, props.Namespace, sname)
	if err != nil {
		logger.Error(err, "Failed to get secret", "name", sname, "namespace", props.Namespace)
		return nil
	}

	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook cnofiguration, check if the corresponding secret is created
	annotations := unstrSecret.GetAnnotations()
	if _, ok := annotations[selfSignedAnnotation]; ok {
		sname := generateRootCASecretName(props)
		_, err := c.GetResource(ctx, "", Secrets, props.Namespace, sname)
		if err != nil {
			logger.Error(err, "Root CA secret is required while using self-signed certificates TLS pair, defaulting to generating new TLS pair", "name", sname, "namespace", props.Namespace)
			return nil
		}
	}
	secret, err := convertToSecret(unstrSecret)
	if err != nil {
		return nil
	}
	pemPair := tls.TlsPemPair{
		Certificate: secret.Data[v1.TLSCertKey],
		PrivateKey:  secret.Data[v1.TLSPrivateKeyKey],
	}
	if len(pemPair.Certificate) == 0 {
		logger.Info("TLS Certificate not found in secret", "name", sname, "namespace", props.Namespace)
		return nil
	}
	if len(pemPair.PrivateKey) == 0 {
		logger.Info("TLS PrivateKey not found in secret", "name", sname, "namespace", props.Namespace)
		return nil
	}
	return &pemPair
}

//WriteTlsPair Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (c *Client) WriteTlsPair(ctx context.Context, props tls.TlsCertificateProps, pemPair *tls.TlsPemPair) error {
	logger := c.log.WithName("WriteTlsPair")
	name := generateTLSPairSecretName(props)
	_, err := c.GetResource(ctx, "", Secrets, props.Namespace, name)
	if err != nil {
		secret := &v1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: props.Namespace,
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       pemPair.Certificate,
				v1.TLSPrivateKeyKey: pemPair.PrivateKey,
			},
			Type: v1.SecretTypeTLS,
		}

		_, err := c.CreateResource(ctx, "", Secrets, props.Namespace, secret, false)
		if err == nil {
			logger.Info("secret created", "name", name, "namespace", props.Namespace)
		}
		return err
	}
	secret := v1.Secret{}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[v1.TLSCertKey] = pemPair.Certificate
	secret.Data[v1.TLSPrivateKeyKey] = pemPair.PrivateKey

	_, err = c.UpdateResource(ctx, "", Secrets, props.Namespace, secret, false)
	if err != nil {
		return err
	}
	logger.Info("secret updated", "name", name, "namespace", props.Namespace)
	return nil
}

func generateTLSPairSecretName(props tls.TlsCertificateProps) string {
	return tls.GenerateInClusterServiceName(props) + ".kyverno-tls-pair"
}

func generateRootCASecretName(props tls.TlsCertificateProps) string {
	return tls.GenerateInClusterServiceName(props) + ".kyverno-tls-ca"
}

//GetTLSCertProps provides the TLS Certificate Properties
func (c *Client) GetTLSCertProps(configuration *rest.Config) (certProps tls.TlsCertificateProps, err error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return certProps, err
	}
	certProps = tls.TlsCertificateProps{
		Service:       config.WebhookServiceName,
		Namespace:     config.KubePolicyNamespace,
		ApiServerHost: apiServerURL.Hostname(),
	}
	return certProps, nil
}
