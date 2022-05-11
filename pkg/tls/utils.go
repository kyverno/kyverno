package tls

import (
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

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
	labels := secret.GetLabels()
	if labels == nil {
		return false
	}
	if labels[ManagedByLabel] != "kyverno" {
		return false
	}
	return true
}
