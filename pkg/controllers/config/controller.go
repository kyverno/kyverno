package config

import (
	"github.com/kyverno/kyverno/pkg/config"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
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

func NewController(configuration config.Configuration, configmapInformer corev1informers.ConfigMapInformer) *controller {
	c := controller{
		configuration:   configuration,
		configmapLister: configmapInformer.Lister(),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "config-controller"),
	}
	controllerutils.AddDefaultEventHandlers(logger, configmapInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(logger, c.queue, workers, maxRetries, c.reconcile, stopCh)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger.Info("reconciling ...", "key", key, "namespace", namespace, "name", name)
	if namespace != config.KyvernoNamespace() || name != config.KyvernoConfigMapName() {
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
