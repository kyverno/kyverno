package tls

import (
	"fmt"
	"net/url"

	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

var ErrorsNotFound = "root CA certificate not found"

// ReadRootCASecret returns the RootCA from the pre-defined secret
func ReadRootCASecret(restConfig *rest.Config, client *client.Client) (result []byte, err error) {
	certProps, err := GetTLSCertProps(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TLS Cert Properties")
	}

	sname := generateRootCASecretName(certProps)
	stlsca, err := client.GetResource("", "Secret", certProps.Namespace, sname)
	if err != nil {
		return nil, err
	}

	tlsca, err := convertToSecret(stlsca)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert secret %s/%s", certProps.Namespace, sname)
	}

	result = tlsca.Data[RootCAKey]
	if len(result) == 0 {
		return nil, errors.Errorf("%s in secret %s/%s", ErrorsNotFound, certProps.Namespace, tlsca.Name)
	}

	return result, nil
}

// ReadTLSPair returns the pem pair from the pre-defined secret
func ReadTLSPair(restConfig *rest.Config, client *client.Client) (*PemPair, error) {
	certProps, err := GetTLSCertProps(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TLS Cert Properties")
	}

	sname := generateTLSPairSecretName(certProps)
	unstrSecret, err := client.GetResource("", "Secret", certProps.Namespace, sname)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %v", certProps.Namespace, sname, err)
	}

	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook configuration, check if the corresponding secret is created
	annotations := unstrSecret.GetAnnotations()
	if _, ok := annotations[SelfSignedAnnotation]; ok {
		sname := generateRootCASecretName(certProps)
		_, err := client.GetResource("", "Secret", certProps.Namespace, sname)
		if err != nil {
			return nil, fmt.Errorf("rootCA secret is required while using self-signed certificate TLS pair, defaulting to generating new TLS pair  %s/%s", certProps.Namespace, sname)
		}
	}
	secret, err := convertToSecret(unstrSecret)
	if err != nil {
		return nil, err
	}

	pemPair := PemPair{
		Certificate: secret.Data[v1.TLSCertKey],
		PrivateKey:  secret.Data[v1.TLSPrivateKeyKey],
	}

	if len(pemPair.Certificate) == 0 {
		return nil, fmt.Errorf("TLS Certificate not found in secret %s/%s", certProps.Namespace, sname)
	}
	if len(pemPair.PrivateKey) == 0 {
		return nil, fmt.Errorf("TLS PrivateKey not found in secret %s/%s", certProps.Namespace, sname)
	}

	return &pemPair, nil
}

//GetTLSCertProps provides the TLS Certificate Properties
func GetTLSCertProps(configuration *rest.Config) (certProps CertificateProps, err error) {
	apiServerURL, err := url.Parse(configuration.Host)
	if err != nil {
		return certProps, err
	}

	certProps = CertificateProps{
		Service:       config.KyvernoServiceName,
		Namespace:     config.KyvernoNamespace,
		APIServerHost: apiServerURL.Hostname(),
	}
	return certProps, nil
}

func convertToSecret(obj *unstructured.Unstructured) (v1.Secret, error) {
	secret := v1.Secret{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &secret); err != nil {
		return secret, err
	}
	return secret, nil
}
