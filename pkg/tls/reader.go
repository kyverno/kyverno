package tls

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var errorsNotFound = "root CA certificate not found"

// ReadRootCASecret returns the RootCA from the pre-defined secret
func ReadRootCASecret(name, namespace string, client corev1listers.SecretNamespaceLister) ([]byte, error) {
	stlsca, err := client.Get(name)
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
		return nil, fmt.Errorf("%s in secret %s/%s", errorsNotFound, namespace, stlsca.Name)
	}
	return result, nil
}
