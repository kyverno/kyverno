package tls

import (
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var ErrorsNotFound = "root CA certificate not found"

// ReadRootCASecret returns the RootCA from the pre-defined secret
func ReadRootCASecret(client corev1listers.SecretNamespaceLister) ([]byte, error) {
	sname := GenerateRootCASecretName()
	stlsca, err := client.Get(sname)
	if err != nil {
		return nil, err
	}
	// try "tls.crt"
	result := stlsca.Data[corev1.TLSCertKey]
	// if not there, try old "rootCA.crt"
	if len(result) == 0 {
		result = stlsca.Data[rootCAKey]
	}
	if len(result) == 0 {
		return nil, errors.Errorf("%s in secret %s/%s", ErrorsNotFound, config.KyvernoNamespace(), stlsca.Name)
	}
	return result, nil
}
