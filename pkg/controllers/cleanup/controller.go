package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	kyvernov1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernov1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	// clients
	client kubernetes.Interface

	// listers
	cpolLister kyvernov1alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov1alpha1listers.CleanupPolicyLister

	// queue
	queue workqueue.RateLimitingInterface
}

const (
	maxRetries     = 10
	Workers        = 3
	ControllerName = "cleanup-controller"
)

func NewController(
	client kubernetes.Interface,
	cpolInformer kyvernov1alpha1informers.ClusterCleanupPolicyInformer,
	polInformer kyvernov1alpha1informers.CleanupPolicyInformer,
) *Controller {
	c := &Controller{
		client:     client,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	return c
}

func (c *Controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *Controller) getPolicy(namespace, name string) (kyvernov1alpha1.CleanupPolicyInterface, error) {
	if namespace == "" {
		cpolicy, err := c.cpolLister.Get(name)
		if err != nil {
			return nil, err
		}
		return cpolicy, nil
	} else {
		policy, err := c.polLister.CleanupPolicies(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return policy, nil
	}
}

func (c *Controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.getPolicy(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "unable to get the policy from policy informer")
		return err
	}

	cronjob := getCronJobForTriggerResource(policy)
	_, err = c.client.BatchV1().CronJobs("default").Create(ctx, cronjob, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "unable to create the resource of kind CronJob for CleanupPolicy %s", name)
		return err
	}
	return nil
}
