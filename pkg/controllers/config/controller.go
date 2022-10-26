package config

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 3
	ControllerName = "config-controller"
	maxRetries     = 10
)

type controller struct {
	configuration config.Configuration

	// listers
	configmapLister corev1listers.ConfigMapLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(configuration config.Configuration, configmapInformer corev1informers.ConfigMapInformer) controllers.Controller {
	c := controller{
		configuration:   configuration,
		configmapLister: configmapInformer.Lister(),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, configmapInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
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
