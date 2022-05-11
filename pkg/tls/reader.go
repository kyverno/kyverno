package tls

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrorsNotFound = "root CA certificate not found"

// ReadRootCASecret returns the RootCA from the pre-defined secret
func ReadRootCASecret(restConfig *rest.Config, client kubernetes.Interface) ([]byte, error) {
	sname := GenerateRootCASecretName()
	stlsca, err := client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), sname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// try "tls.crt"
	result := stlsca.Data[v1.TLSCertKey]
	// if not there, try old "rootCA.crt"
	if len(result) == 0 {
		result = stlsca.Data[rootCAKey]
	}
	if len(result) == 0 {
		return nil, errors.Errorf("%s in secret %s/%s", ErrorsNotFound, config.KyvernoNamespace(), stlsca.Name)
	}
	return result, nil
}

// ReadTLSPair returns the pem pair from the pre-defined secret
func ReadTLSPair(restConfig *rest.Config, client kubernetes.Interface) ([]byte, []byte, error) {
	sname := GenerateTLSPairSecretName()
	secret, err := client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), sname, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret %s/%s: %v", config.KyvernoNamespace(), sname, err)
	}
	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook configuration, check if the corresponding secret is created
	{
		sname := GenerateRootCASecretName()
		_, err := client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), sname, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("rootCA secret is required while using self-signed certificate TLS pair, defaulting to generating new TLS pair  %s/%s", config.KyvernoNamespace(), sname)
		}
	}
	if len(secret.Data[v1.TLSCertKey]) == 0 {
		return nil, nil, fmt.Errorf("TLS Certificate not found in secret %s/%s", config.KyvernoNamespace(), sname)
	}
	if len(secret.Data[v1.TLSPrivateKeyKey]) == 0 {
		return nil, nil, fmt.Errorf("TLS PrivateKey not found in secret %s/%s", config.KyvernoNamespace(), sname)
	}
	return secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey], nil
}
