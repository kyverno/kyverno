package tls

import (
	"context"
	"fmt"
	"net/url"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrorsNotFound = "root CA certificate not found"

// ReadRootCASecret returns the RootCA from the pre-defined secret
func ReadRootCASecret(restConfig *rest.Config, client kubernetes.Interface) (result []byte, err error) {
	certProps, err := GetTLSCertProps(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TLS Cert Properties")
	}

	depl, err := client.AppsV1().Deployments(certProps.Namespace).Get(context.TODO(), config.KyvernoDeploymentName, metav1.GetOptions{})

	deplHash := ""
	if err == nil {
		deplHash = fmt.Sprintf("%v", depl.GetUID())
	}

	var deplHashSec string = "default"
	var ok, managedByKyverno bool

	sname := GenerateRootCASecretName(certProps)
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

	result = stlsca.Data[RootCAKey]
	if len(result) == 0 {
		return nil, errors.Errorf("%s in secret %s/%s", ErrorsNotFound, certProps.Namespace, stlsca.Name)
	}

	return result, nil
}

// ReadTLSPair returns the pem pair from the pre-defined secret
func ReadTLSPair(restConfig *rest.Config, client kubernetes.Interface) (*PemPair, error) {
	certProps, err := GetTLSCertProps(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TLS Cert Properties")
	}

	depl, err := client.AppsV1().Deployments(certProps.Namespace).Get(context.TODO(), config.KyvernoDeploymentName, metav1.GetOptions{})

	deplHash := ""
	if err == nil {
		deplHash = fmt.Sprintf("%v", depl.GetUID())
	}

	var deplHashSec string = "default"
	var ok, managedByKyverno bool

	sname := GenerateTLSPairSecretName(certProps)
	secret, err := client.CoreV1().Secrets(certProps.Namespace).Get(context.TODO(), sname, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %v", certProps.Namespace, sname, err)
	}
	if label, ok := secret.GetLabels()[ManagedByLabel]; ok {
		managedByKyverno = label == "kyverno"
	}
	deplHashSec, ok = secret.GetAnnotations()[MasterDeploymentUID]
	if managedByKyverno && (ok && deplHashSec != deplHash) {
		return nil, fmt.Errorf("outdated secret")
	}

	// If secret contains annotation 'self-signed-cert', then it's created using helper scripts to setup self-signed certificates.
	// As the root CA used to sign the certificate is required for webhook configuration, check if the corresponding secret is created
	annotations := secret.GetAnnotations()
	if _, ok := annotations[SelfSignedAnnotation]; ok {
		sname := GenerateRootCASecretName(certProps)
		_, err := client.CoreV1().Secrets(certProps.Namespace).Get(context.TODO(), sname, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("rootCA secret is required while using self-signed certificate TLS pair, defaulting to generating new TLS pair  %s/%s", certProps.Namespace, sname)
		}
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
