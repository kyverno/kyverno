package runtime

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tls"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
)

type Runtime interface {
	IsDebug() bool
	IsReady(ctx context.Context) bool
	IsLive(ctx context.Context) bool
	IsRollingUpdate() bool
	IsGoingDown() bool
}

type runtime struct {
	serverIP         string
	deploymentLister appsv1listers.DeploymentLister
	certValidator    tls.CertValidator
	logger           logr.Logger
}

func NewRuntime(
	logger logr.Logger,
	serverIP string,
	deploymentInformer appsv1informers.DeploymentInformer,
	certValidator tls.CertValidator,
) Runtime {
	return &runtime{
		logger:           logger,
		serverIP:         serverIP,
		deploymentLister: deploymentInformer.Lister(),
		certValidator:    certValidator,
	}
}

func (c *runtime) IsDebug() bool {
	return c.serverIP != ""
}

func (c *runtime) IsLive(context.Context) bool {
	return true
}

func (c *runtime) IsReady(ctx context.Context) bool {
	return c.validateCertificates(ctx)
}

func (c *runtime) IsRollingUpdate() bool {
	if c.IsDebug() {
		return false
	}
	deployment, err := c.getDeployment()
	if err != nil {
		c.logger.Error(err, "failed to get deployment")
		return true
	}
	var replicas int32 = 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	nonTerminatedReplicas := deployment.Status.Replicas
	if nonTerminatedReplicas > replicas {
		c.logger.V(2).Info("detect Kyverno is in rolling update, won't trigger the update again")
		return true
	}
	return false
}

func (c *runtime) IsGoingDown() bool {
	if c.IsDebug() {
		return false
	}
	deployment, err := c.getDeployment()
	if err != nil {
		return apierrors.IsNotFound(err)
	}
	if deployment.GetDeletionTimestamp() != nil {
		return true
	}
	if deployment.Spec.Replicas != nil {
		return *deployment.Spec.Replicas == 0
	}
	return false
}

func (c *runtime) getDeployment() (*appsv1.Deployment, error) {
	return c.deploymentLister.Deployments(config.KyvernoNamespace()).Get(config.KyvernoDeploymentName())
}

func (c *runtime) validateCertificates(ctx context.Context) bool {
	validity, err := c.certValidator.ValidateCert(ctx)
	if err != nil {
		c.logger.Error(err, "failed to validate certificates")
		return false
	}
	return validity
}
