package tls

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

// PrivateKeyToPem Creates PEM block from private key object
func PrivateKeyToPem(rsaKey *rsa.PrivateKey) []byte {
	privateKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}
	return pem.EncodeToMemory(privateKey)
}

// CertificateToPem Creates PEM block from certificate object
func CertificateToPem(cert *x509.Certificate) []byte {
	certificate := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(certificate)
}

// IsKyvernoInRollingUpdate returns true if Kyverno is in rolling update
func IsKyvernoInRollingUpdate(deploy *appsv1.Deployment, logger logr.Logger) bool {
	var replicas int32 = 1
	if deploy.Spec.Replicas != nil {
		replicas = *deploy.Spec.Replicas
	}
	nonTerminatedReplicas := deploy.Status.Replicas
	if nonTerminatedReplicas > replicas {
		logger.Info("detect Kyverno is in rolling update, won't trigger the update again")
		return true
	}
	return false
}

func IsSecretManagedByKyverno(secret *v1.Secret) bool {
	if secret != nil {
		labels := secret.GetLabels()
		if labels == nil {
			return false
		}
		if labels[ManagedByLabel] != "kyverno" {
			return false
		}
	}
	return true
}

// InClusterServiceName The generated service name should be the common name for TLS certificate
func InClusterServiceName() string {
	return config.KyvernoServiceName() + "." + config.KyvernoNamespace() + ".svc"
}

func GenerateTLSPairSecretName() string {
	return InClusterServiceName() + ".kyverno-tls-pair"
}

func GenerateRootCASecretName() string {
	return InClusterServiceName() + ".kyverno-tls-ca"
}
