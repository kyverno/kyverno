package cleanup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernov1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha1"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	client kubernetes.Clientset
	// listers
	cpolLister kyvernov1alpha1listers.ClusterCleanupPolicyLister
	polLister  kyvernov1alpha1listers.CleanupPolicyLister

	// queue
	queue workqueue.RateLimitingInterface
}

const (
	MaxRetries = 10
	Workers    = 3
)

func NewController(kubeClient kubernetes.Clientset, cpolInformer kyvernov1alpha1informers.ClusterCleanupPolicyInformer, polInformer kyvernov1alpha1informers.CleanupPolicyInformer) *Controller {
	c := &Controller{
		client:     kubeClient,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cleanup-controller"),
	}

	cpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj)
		},
	})
	polInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj)
		},
	})
	return c
}

func (c *Controller) enqueue(obj interface{}) {
	c.queue.Add(obj)
}

func (c *Controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, Workers, MaxRetries, c.reconcile)
}

func (c *Controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
	policy, err := c.getPolicy(namespace, name)
	if err != nil {
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
