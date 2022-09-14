package report

import (
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// listers
	rcrInformer  kyvernov1alpha2informers.ReportChangeRequestInformer
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer

	// // configmapSynced returns true if the configmap shared informer has synced at least once
	// configmapSynced cache.InformerSynced

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer, crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer) *controller {
	c := controller{
		rcrInformer:  rcrInformer,
		crcrInformer: crcrInformer,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, rcrInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, crcrInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")
	return nil
}
