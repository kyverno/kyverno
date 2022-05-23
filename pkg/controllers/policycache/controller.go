package policycache

import (
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	pcache "github.com/kyverno/kyverno/pkg/policycache"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	cache pcache.Cache

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(pcache pcache.Cache, cpolInformer kyvernov1informers.ClusterPolicyInformer, polInformer kyvernov1informers.PolicyInformer) *controller {
	c := controller{
		cache:      pcache,
		cpolLister: cpolInformer.Lister(),
		polLister:  polInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policycache-controller"),
	}
	cpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.add,
		UpdateFunc: c.update,
		DeleteFunc: c.delete,
	})
	polInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.add,
		UpdateFunc: c.update,
		DeleteFunc: c.delete,
	})
	return &c
}

func (c *controller) add(obj interface{}) {
	c.enqueue(obj)
}

func (c *controller) update(_, cur interface{}) {
	c.enqueue(cur)
}

func (c *controller) delete(obj interface{}) {
	c.enqueue(kubeutils.GetObjectWithTombstone(obj))
}

func (c *controller) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		logger.Error(err, "failed to compute key name")
	} else {
		c.queue.Add(key)
	}
}

func (c *controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
	} else if errors.IsNotFound(err) {
		logger.V(4).Info("Dropping request from the queue", "key", key, "error", err.Error())
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("Retrying request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
	} else {
		logger.Error(err, "Failed to process request", "key", key)
		c.queue.Forget(key)
	}
}

func (c *controller) processNextWorkItem() bool {
	if key, quit := c.queue.Get(); !quit {
		defer c.queue.Done(key)
		c.handleErr(c.reconcile(key.(string)), key)
		return true
	}
	return false
}

func (c *controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) WarmUp() error {
	logger.Info("warming up ...")
	defer logger.Info("warm up done")
	pols, err := c.polLister.Policies(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, policy := range pols {
		if key, err := cache.MetaNamespaceKeyFunc(policy); err != nil {
			return err
		} else {
			c.cache.Set(key, policy)
		}
	}
	cpols, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, policy := range cpols {
		if key, err := cache.MetaNamespaceKeyFunc(policy); err != nil {
			return err
		} else {
			c.cache.Set(key, policy)
		}
	}
	return nil
}

func (c *controller) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	logger.Info("starting ...")
	defer logger.Info("shutting down")
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *controller) reconcile(key string) error {
	logger.Info("reconciling ...", "key", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	policy, err := c.loadPolicy(namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.cache.Unset(key)
		}
		return err
	}
	// TODO: check resource version ?
	c.cache.Set(key, policy)
	return nil
}

func (c *controller) loadPolicy(namespace, name string) (kyvernov1.PolicyInterface, error) {
	if namespace == "" {
		return c.cpolLister.Get(name)
	} else {
		return c.polLister.Policies(namespace).Get(name)
	}
}
