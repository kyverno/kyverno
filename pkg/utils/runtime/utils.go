package runtime

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tls"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	coordinationv1informers "k8s.io/client-go/informers/coordination/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
)

const (
	AnnotationLastRequestTime = "kyverno.io/last-request-time"
	IdleDeadline              = tickerInterval * 5
	tickerInterval            = 30 * time.Second
)

type Runtime interface {
	IsDebug() bool
	IsReady() bool
	IsLive() bool
	IsRollingUpdate() bool
	IsGoingDown() bool
}

type runtime struct {
	serverIP         string
	leaseLister      coordinationv1listers.LeaseLister
	deploymentLister appsv1listers.DeploymentLister
	certValidator    tls.CertValidator
	logger           logr.Logger
}

func NewRuntime(
	logger logr.Logger,
	serverIP string,
	leaseInformer coordinationv1informers.LeaseInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	certValidator tls.CertValidator,
) Runtime {
	return &runtime{
		logger:           logger,
		serverIP:         serverIP,
		leaseLister:      leaseInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		certValidator:    certValidator,
	}
}

func (c *runtime) IsDebug() bool {
	return c.serverIP != ""
}

func (c *runtime) IsLive() bool {
	return c.check()
}

func (c *runtime) IsReady() bool {
	return c.check() && c.validateCertificates()
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
		c.logger.Info("detect Kyverno is in rolling update, won't trigger the update again")
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

func (c *runtime) getLease() (*coordinationv1.Lease, error) {
	return c.leaseLister.Leases(config.KyvernoNamespace()).Get("kyverno")
}

func (c *runtime) getDeployment() (*appsv1.Deployment, error) {
	return c.deploymentLister.Deployments(config.KyvernoNamespace()).Get(config.KyvernoDeploymentName())
}

func (c *runtime) check() bool {
	lease, err := c.getLease()
	if err != nil {
		c.logger.Error(err, "failed to get lease")
		return false
	}
	annotations := lease.GetAnnotations()
	if annotations == nil {
		return false
	}
	annTime, err := time.Parse(time.RFC3339, annotations[AnnotationLastRequestTime])
	if err != nil {
		return false
	}
	return time.Now().Before(annTime.Add(IdleDeadline))
}

func (c *runtime) validateCertificates() bool {
	validity, err := c.certValidator.ValidateCert()
	if err != nil {
		c.logger.Error(err, "failed to validate certificates")
		return false
	}
	return validity
}
