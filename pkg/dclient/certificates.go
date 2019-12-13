package client

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/golang/glog"
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
func (c *Client) InitTLSPemPair(configuration *rest.Config) (*tls.TlsPemPair, error) {
	certProps, err := c.GetTLSCertProps(configuration)
	if err != nil {
		return nil, err
	}
	tlsPair := c.ReadTlsPair(certProps)
	if tls.IsTlsPairShouldBeUpdated(tlsPair) {
		glog.Info("Generating new key/certificate pair for TLS")
		tlsPair, err = c.GenerateTlsPemPair(certProps)
		if err != nil {
			return nil, err
		}
		if err = c.WriteTlsPair(certProps, tlsPair); err != nil {
			return nil, fmt.Errorf("Unable to save TLS pair to the cluster: %v", err)
		}
		return tlsPair, nil
	}

	glog.Infoln("Using existing TLS key/certificate pair")
	return tlsPair, nil
}

//GenerateTlsPemPair Issues TLS certificate for webhook server using given PEM private key
// Returns signed and approved TLS certificate in PEM format
func (c *Client) GenerateTlsPemPair(props tls.TlsCertificateProps) (*tls.TlsPemPair, error) {
	privateKey, err := tls.TlsGeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	certRequest, err := tls.TlsCertificateGenerateRequest(privateKey, props)
	if err != nil {
		return nil, fmt.Errorf("Unable to create certificate request: %v", err)
	}

	certRequest, err = c.submitAndApproveCertificateRequest(certRequest)
	if err != nil {
		return nil, fmt.Errorf("Unable to submit and approve certificate request: %v", err)
	}

	tlsCert, err := c.fetchCertificateFromRequest(certRequest, 10)
	if err != nil {
		return nil, fmt.Errorf("Failed to configure a certificate for the Kyverno controller. A CA certificate is required to allow the Kubernetes API Server to communicate with Kyverno. You can either provide a certificate or configure your cluster to allow certificate signing. Please refer to https://github.com/nirmata/kyverno/installation.md.: %v", err)
	}

	return &tls.TlsPemPair{
		Certificate: tlsCert,
		PrivateKey:  tls.TlsPrivateKeyToPem(privateKey),
	}, nil
}

// Submits and approves certificate request, returns request which need to be fetched
func (c *Client) submitAndApproveCertificateRequest(req *certificates.CertificateSigningRequest) (*certificates.CertificateSigningRequest, error) {
	certClient, err := c.GetCSRInterface()
	if err != nil {
		return nil, err
	}
	csrList, err := c.ListResource(CSRs, "", nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to list existing certificate requests: %v", err)
	}

	for _, csr := range csrList.Items {
		if csr.GetName() == req.ObjectMeta.Name {
			err := c.DeleteResource(CSRs, "", csr.GetName(), false)
			if err != nil {
				return nil, fmt.Errorf("Unable to delete existing certificate request: %v", err)
			}
			glog.Info("Old certificate request is deleted")
			break
		}
	}

	unstrRes, err := c.CreateResource(CSRs, "", req, false)
	if err != nil {
		return nil, err
	}
	glog.Infof("Certificate request %s is created", unstrRes.GetName())

	res, err := convertToCSR(unstrRes)
	if err != nil {
		return nil, err
	}
	res.Status.Conditions = append(res.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "NKP-Approve",
		Message: "This CSR was approved by Nirmata kyverno controller",
	})
	res, err = certClient.UpdateApproval(res)
	if err != nil {
		return nil, fmt.Errorf("Unable to approve certificate request: %v", err)
	}
	glog.Infof("Certificate request %s is approved", res.ObjectMeta.Name)

	return res, nil
}

// Fetches certificate from given request. Tries to obtain certificate for maxWaitSeconds
func (c *Client) fetchCertificateFromRequest(req *certificates.CertificateSigningRequest, maxWaitSeconds uint8) ([]byte, error) {
	// TODO: react of SIGINT and SIGTERM
	timeStart := time.Now()
	for time.Now().Sub(timeStart) < time.Duration(maxWaitSeconds)*time.Second {
		unstrR, err := c.GetResource(CSRs, "", req.ObjectMeta.Name)
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
func (c *Client) ReadRootCASecret() (result []byte) {
	certProps, err := c.GetTLSCertProps(c.clientConfig)
	if err != nil {
		glog.Error(err)
		return result
	}
	sname := generateRootCASecretName(certProps)
	stlsca, err := c.GetResource(Secrets, certProps.Namespace, sname)
	if err != nil {
		return result
	}
	tlsca, err := convertToSecret(stlsca)
	if err != nil {
		glog.Error(err)
		return result
	}

	result = tlsca.Data[rootCAKey]
	if len(result) == 0 {
		glog.Warningf("root CA certificate not found in secret %s/%s", certProps.Namespace, tlsca.Name)
		return result
	}
	glog.V(4).Infof("using CA bundle defined in secret %s/%s to validate the webhook's server certificate", certProps.Namespace, tlsca.Name)
	return result
}

const selfSignedAnnotation string = "self-signed-cert"
const rootCAKey string = "rootCA.crt"

//ReadTlsPair Reads the pair of TLS certificate and key from the specified secret.
func (c *Client) ReadTlsPair(props tls.TlsCertificateProps) *tls.TlsPemPair {
	sname := generateTLSPairSecretName(props)
	unstrSecret, err := c.GetResource(Secrets, props.Namespace, sname)
	if err != nil {
		glog.Warningf("Unable to get secret %s/%s: %s", props.Namespace, sname, err)
		return nil
	}

	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook cnofiguration, check if the corresponding secret is created
	annotations := unstrSecret.GetAnnotations()
	if _, ok := annotations[selfSignedAnnotation]; ok {
		sname := generateRootCASecretName(props)
		_, err := c.GetResource(Secrets, props.Namespace, sname)
		if err != nil {
			glog.Errorf("Root CA secret %s/%s is required while using self-signed certificates TLS pair, defaulting to generating new TLS pair", props.Namespace, sname)
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
		glog.Warningf("TLS Certificate not found in secret %s/%s", props.Namespace, sname)
		return nil
	}
	if len(pemPair.PrivateKey) == 0 {
		glog.Warningf("TLS PrivateKey not found in secret %s/%s", props.Namespace, sname)
		return nil
	}
	return &pemPair
}

//WriteTlsPair Writes the pair of TLS certificate and key to the specified secret.
// Updates existing secret or creates new one.
func (c *Client) WriteTlsPair(props tls.TlsCertificateProps, pemPair *tls.TlsPemPair) error {
	name := generateTLSPairSecretName(props)
	_, err := c.GetResource(Secrets, props.Namespace, name)
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

		_, err := c.CreateResource(Secrets, props.Namespace, secret, false)
		if err == nil {
			glog.Infof("Secret %s is created", name)
		}
		return err
	}
	secret := v1.Secret{}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[v1.TLSCertKey] = pemPair.Certificate
	secret.Data[v1.TLSPrivateKeyKey] = pemPair.PrivateKey

	_, err = c.UpdateResource(Secrets, props.Namespace, secret, false)
	if err != nil {
		return err
	}
	glog.Infof("Secret %s is updated", name)
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
