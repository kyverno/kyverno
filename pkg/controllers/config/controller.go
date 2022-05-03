package config

import (
	"time"

	"github.com/kyverno/kyverno/pkg/config"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	configuration config.Configuration

	// listers
	configmapLister corev1listers.ConfigMapLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(configmapInformer corev1informers.ConfigMapInformer, configuration config.Configuration) *controller {
	c := controller{
		configuration:   configuration,
		configmapLister: configmapInformer.Lister(),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "config-controller"),
	}
	configmapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.add,
		UpdateFunc: c.update,
		DeleteFunc: c.delete,
	})
	return &c
}

func (c *controller) add(obj interface{}) {
	c.enqueue(obj.(*corev1.ConfigMap))
}

func (c *controller) update(old, cur interface{}) {
	c.enqueue(cur.(*corev1.ConfigMap))
}

func (c *controller) delete(obj interface{}) {
	cm, ok := kubeutils.GetObjectWithTombstone(obj).(*corev1.ConfigMap)
	if ok {
		c.enqueue(cm)
	} else {
		logger.Info("Failed to get deleted object", "obj", obj)
	}
}

func (c *controller) enqueue(obj *corev1.ConfigMap) {
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
		logger.V(4).Info("Dropping update request from the queue", "key", key, "error", err.Error())
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying update request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
	} else {
		logger.Error(err, "failed to process update request", "key", key)
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

func (c *controller) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	logger.Info("start")
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
	if namespace != config.KyvernoNamespace || name != config.KyvernoConfigMapName {
		return nil
	}
	configMap, err := c.configmapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.configuration.Load(nil)
		}
		return err
	}
	c.configuration.Load(configMap.DeepCopy())
	return nil
}
