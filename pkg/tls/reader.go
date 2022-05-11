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
	certProps, err := NewCertificateProps(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TLS Cert Properties")
	}

	depl, err := client.AppsV1().Deployments(certProps.Namespace).Get(context.TODO(), config.KyvernoDeploymentName(), metav1.GetOptions{})

	deplHash := ""
	if err == nil {
		deplHash = fmt.Sprintf("%v", depl.GetUID())
	}

	var deplHashSec string
	var ok, managedByKyverno bool

	sname := certProps.GenerateRootCASecretName()
	stlsca, err := client.CoreV1().Secrets(certProps.Namespace).Get(context.TODO(), sname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if label, ok := stlsca.GetLabels()[ManagedByLabel]; ok {
		managedByKyverno = label == "kyverno"
	}
	deplHashSec, ok = stlsca.GetAnnotations()[MasterDeploymentUID]
	if managedByKyverno && (ok && deplHashSec != deplHash) {
		return nil, fmt.Errorf("outdated secret")
	}

	if len(stlsca.Data[RootCAKey]) == 0 {
		return nil, errors.Errorf("%s in secret %s/%s", ErrorsNotFound, certProps.Namespace, stlsca.Name)
	}

	return stlsca.Data[RootCAKey], nil
}

// ReadTLSPair returns the pem pair from the pre-defined secret
func ReadTLSPair(restConfig *rest.Config, client kubernetes.Interface) ([]byte, []byte, error) {
	certProps, err := NewCertificateProps(restConfig)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get TLS Cert Properties")
	}

	depl, err := client.AppsV1().Deployments(certProps.Namespace).Get(context.TODO(), config.KyvernoDeploymentName(), metav1.GetOptions{})

	deplHash := ""
	if err == nil {
		deplHash = fmt.Sprintf("%v", depl.GetUID())
	}

	var deplHashSec string
	var ok, managedByKyverno bool

	sname := certProps.GenerateTLSPairSecretName()
	secret, err := client.CoreV1().Secrets(certProps.Namespace).Get(context.TODO(), sname, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret %s/%s: %v", certProps.Namespace, sname, err)
	}
	if label, ok := secret.GetLabels()[ManagedByLabel]; ok {
		managedByKyverno = label == "kyverno"
	}
	deplHashSec, ok = secret.GetAnnotations()[MasterDeploymentUID]
	if managedByKyverno && (ok && deplHashSec != deplHash) {
		return nil, nil, fmt.Errorf("outdated secret")
	}

	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook configuration, check if the corresponding secret is created
	{
		sname := certProps.GenerateRootCASecretName()
		_, err := client.CoreV1().Secrets(certProps.Namespace).Get(context.TODO(), sname, metav1.GetOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("rootCA secret is required while using self-signed certificate TLS pair, defaulting to generating new TLS pair  %s/%s", certProps.Namespace, sname)
		}
	}

	if len(secret.Data[v1.TLSCertKey]) == 0 {
		return nil, nil, fmt.Errorf("TLS Certificate not found in secret %s/%s", certProps.Namespace, sname)
	}
	if len(secret.Data[v1.TLSPrivateKeyKey]) == 0 {
		return nil, nil, fmt.Errorf("TLS PrivateKey not found in secret %s/%s", certProps.Namespace, sname)
	}

	return secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey], nil
}
