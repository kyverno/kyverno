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

type controller struct {
	// clients
	client kubernetes.Interface

	// listers
	cpolLister kyvernov1alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov1alpha1listers.CleanupPolicyLister
	cjLister   batchv1listers.CronJobLister

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
	cjInformer batchv1informers.CronJobInformer,
) controllers.Controller {
	c := &controller{
		client:     client,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		cjLister:   cjInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	cpolEnqueue := controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	polEnqueue := controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddEventHandlersT(
		cjInformer.Informer(),
		func(n *batchv1.CronJob) {
			if len(n.OwnerReferences) == 1 {
				if n.OwnerReferences[0].Kind == "ClusterCleanupPolicy" {
					cpol := kyvernov1alpha1.ClusterCleanupPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name: n.OwnerReferences[0].Name,
						},
					}
					err := cpolEnqueue(&cpol)
					if err != nil {
						logger.Error(err, "failed to enqueue ClusterCleanupPolicy object", cpol)
					}
				} else if n.OwnerReferences[0].Kind == "CleanupPolicy" {
					pol := kyvernov1alpha1.CleanupPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      n.OwnerReferences[0].Name,
							Namespace: n.Namespace,
						},
					}
					err := polEnqueue(&pol)
					if err != nil {
						logger.Error(err, "failed to enqueue CleanupPolicy object", pol)
					}
				}
			}
		},
		func(o *batchv1.CronJob, n *batchv1.CronJob) {
			if o.GetResourceVersion() != n.GetResourceVersion() {
				for _, owner := range n.OwnerReferences {
					if owner.Kind == "ClusterCleanupPolicy" {
						cpol := kyvernov1alpha1.ClusterCleanupPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name: owner.Name,
							},
						}
						err := cpolEnqueue(&cpol)
						if err != nil {
							logger.Error(err, "failed to enqueue ClusterCleanupPolicy object", cpol)
						}
					} else if owner.Kind == "CleanupPolicy" {
						pol := kyvernov1alpha1.CleanupPolicy{
							ObjectMeta: metav1.ObjectMeta{
								Name:      owner.Name,
								Namespace: n.Namespace,
							},
						}
						err := polEnqueue(&pol)
						if err != nil {
							logger.Error(err, "failed to enqueue CleanupPolicy object", pol)
						}
					}
				}
			}
		},
		func(n *batchv1.CronJob) {
			if len(n.OwnerReferences) == 1 {
				if n.OwnerReferences[0].Kind == "ClusterCleanupPolicy" {
					cpol := kyvernov1alpha1.ClusterCleanupPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name: n.OwnerReferences[0].Name,
						},
					}
					err := cpolEnqueue(&cpol)
					if err != nil {
						logger.Error(err, "failed to enqueue ClusterCleanupPolicy object", cpol)
					}
				} else if n.OwnerReferences[0].Kind == "CleanupPolicy" {
					pol := kyvernov1alpha1.CleanupPolicy{
						ObjectMeta: metav1.ObjectMeta{
							Name:      n.OwnerReferences[0].Name,
							Namespace: n.Namespace,
						},
					}
					err := polEnqueue(&pol)
					if err != nil {
						logger.Error(err, "failed to enqueue CleanupPolicy object", pol)
					}
				}
			}
		},
	)
	return c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
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
	if cronjob, err := c.getCronjob(cronjobNs, string(policy.GetUID())); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		cronjob := getCronJobForTriggerResource(policy)
		_, err = c.client.BatchV1().CronJobs(cronjobNs).Create(ctx, cronjob, metav1.CreateOptions{})
		return err
	} else {
		_, err = controllerutils.Update(ctx, cronjob, c.client.BatchV1().CronJobs(cronjobNs), func(cronjob *batchv1.CronJob) error {
			cronjob.Spec.Schedule = policy.GetSpec().Schedule
			return nil
		})
		return err
	}
}
