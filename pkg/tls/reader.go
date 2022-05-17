package tls

import (
	"context"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var ErrorsNotFound = "root CA certificate not found"

// ReadRootCASecret returns the RootCA from the pre-defined secret
func ReadRootCASecret(client kubernetes.Interface) ([]byte, error) {
	sname := GenerateRootCASecretName()
	stlsca, err := client.CoreV1().Secrets(config.KyvernoNamespace()).Get(context.TODO(), sname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// try "tls.crt"
	result := stlsca.Data[corev1.TLSCertKey]
	// if not there, try old "rootCA.crt"
	if len(result) == 0 {
		result = stlsca.Data[RootCAKey]
	}
	if len(result) == 0 {
		return nil, errors.Errorf("%s in secret %s/%s", ErrorsNotFound, config.KyvernoNamespace(), stlsca.Name)
	}
	return result, nil
}
