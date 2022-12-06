package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	kyvernov1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernov1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	"k8s.io/client-go/kubernetes"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	// CleanupServicePath is the path for triggering cleanup
	CleanupServicePath = "/cleanup"
)

type controller struct {
	// clients
	client kubernetes.Interface

	// listers
	cpolLister kyvernov1alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov1alpha1listers.CleanupPolicyLister
	cjLister   batchv1listers.CronJobLister

	// queue
	queue       workqueue.RateLimitingInterface
	cpolEnqueue controllerutils.EnqueueFuncT[*kyvernov1alpha1.ClusterCleanupPolicy]
	polEnqueue  controllerutils.EnqueueFuncT[*kyvernov1alpha1.CleanupPolicy]

	// config
	cleanupService string
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
	cjInformer batchv1informers.CronJobInformer,
	cleanupService string,
) controllers.Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := &controller{
		client:         client,
		cpolLister:     cpolInformer.Lister(),
		polLister:      polInformer.Lister(),
		cjLister:       cjInformer.Lister(),
		queue:          queue,
		cpolEnqueue:    controllerutils.AddDefaultEventHandlersT[*kyvernov1alpha1.ClusterCleanupPolicy](logger, cpolInformer.Informer(), queue),
		polEnqueue:     controllerutils.AddDefaultEventHandlersT[*kyvernov1alpha1.CleanupPolicy](logger, polInformer.Informer(), queue),
		cleanupService: cleanupService,
	}
	controllerutils.AddEventHandlersT(
		cjInformer.Informer(),
		func(n *batchv1.CronJob) { c.enqueueCronJob(n) },
		func(o *batchv1.CronJob, n *batchv1.CronJob) { c.enqueueCronJob(o) },
		func(n *batchv1.CronJob) { c.enqueueCronJob(n) },
	)
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueueCronJob(n *batchv1.CronJob) {
	if len(n.OwnerReferences) == 1 {
		if n.OwnerReferences[0].Kind == "ClusterCleanupPolicy" {
			cpol := &kyvernov1alpha1.ClusterCleanupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: n.OwnerReferences[0].Name,
				},
			}
			err := c.cpolEnqueue(cpol)
			if err != nil {
				logger.Error(err, "failed to enqueue ClusterCleanupPolicy object", cpol)
			}
		} else if n.OwnerReferences[0].Kind == "CleanupPolicy" {
			pol := &kyvernov1alpha1.CleanupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      n.OwnerReferences[0].Name,
					Namespace: n.Namespace,
				},
			}
			err := c.polEnqueue(pol)
			if err != nil {
				logger.Error(err, "failed to enqueue CleanupPolicy object", pol)
			}
		}
	}
}

func (c *controller) getPolicy(namespace, name string) (kyvernov1alpha1.CleanupPolicyInterface, error) {
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

func (c *controller) getCronjob(namespace, name string) (*batchv1.CronJob, error) {
	cj, err := c.cjLister.CronJobs(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return cj, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.getPolicy(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "unable to get the policy from policy informer")
		return err
	}
	cronjobNs := namespace
	if namespace == "" {
		cronjobNs = config.KyvernoNamespace()
	}
	desired, err := getCronJobForTriggerResource(policy, c.cleanupService)
	if err != nil {
		return err
	}
	if observed, err := c.getCronjob(cronjobNs, string(policy.GetUID())); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		_, err = c.client.BatchV1().CronJobs(cronjobNs).Create(ctx, desired, metav1.CreateOptions{})
		return err
	} else {
		_, err = controllerutils.Update(ctx, observed, c.client.BatchV1().CronJobs(cronjobNs), func(cronjob *batchv1.CronJob) error {
			cronjob.Spec = desired.Spec
			return nil
		})
		return err
	}
}
