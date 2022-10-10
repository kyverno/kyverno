package runtime

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	webhookcontroller "github.com/kyverno/kyverno/pkg/controllers/webhook"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	coordinationv1informers "k8s.io/client-go/informers/coordination/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
)

type Runtime interface {
	IsReady() bool
	IsLive() bool
	IsRollingUpdate() bool
}

type runtime struct {
	serverIP         string
	leaseLister      coordinationv1listers.LeaseLister
	deploymentLister appsv1listers.DeploymentLister
	logger           logr.Logger
}

func NewRuntime(
	logger logr.Logger,
	serverIP string,
	leaseInformer coordinationv1informers.LeaseInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
) Runtime {
	return &runtime{
		serverIP:    serverIP,
		leaseLister: leaseInformer.Lister(),
		logger:      logger,
	}
}

func (c *runtime) getLease() (*coordinationv1.Lease, error) {
	return c.leaseLister.Leases(config.KyvernoNamespace()).Get("kyverno")
}

func (c *runtime) getDeployment() (*appsv1.Deployment, error) {
	return c.deploymentLister.Deployments(config.KyvernoNamespace()).Get("kyverno")
}

func (c *runtime) isDebug() bool {
	return c.serverIP != ""
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
	annTime, err := time.Parse(time.RFC3339, annotations[webhookcontroller.AnnotationLastRequestTime])
	if err != nil {
		return false
	}
	return time.Now().Before(annTime.Add(webhookcontroller.IdleDeadline))
}

func (c *runtime) IsLive() bool {
	return c.isDebug() || c.check()
}

func (c *runtime) IsReady() bool {
	return c.isDebug() || c.check()
}

func (c *runtime) IsRollingUpdate() bool {
	if c.isDebug() {
		return false
	}
	deployment, err := c.getDeployment()
	if err != nil {
		return true
	}
	var replicas int32 = 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	nonTerminatedReplicas := deployment.Status.Replicas
	if nonTerminatedReplicas > replicas {
		// logger.Info("detect Kyverno is in rolling update, won't trigger the update again")
		return true
	}
	return false
}
