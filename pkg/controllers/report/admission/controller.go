package admission

import (
	"time"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 5
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
	admrLister  kyvernov1alpha2listers.AdmissionReportLister
	cadmrLister kyvernov1alpha2listers.ClusterAdmissionReportLister

	// queue
	queue workqueue.RateLimitingInterface

	// cache
	metadataCache resource.MetadataCache
}

func NewController(
	client versioned.Interface,
	admrInformer kyvernov1alpha2informers.AdmissionReportInformer,
	cadmrInformer kyvernov1alpha2informers.ClusterAdmissionReportInformer,
	metadataCache resource.MetadataCache,
) *controller {
	c := controller{
		client:        client,
		admrLister:    admrInformer.Lister(),
		cadmrLister:   cadmrInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		metadataCache: metadataCache,
	}
	controllerutils.AddDefaultEventHandlers(logger, admrInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cadmrInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	c.metadataCache.AddEventHandler(func(_ string, uid types.UID) {
		selector, err := reportutils.SelectorResourceUidEquals(uid)
		if err != nil {
			logger.Error(err, "failed to create label selector")
		}
		if err := c.enqueue(selector); err != nil {
			logger.Error(err, "failed to enqueue")
		}
	})
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) enqueue(selector labels.Selector) error {
	logger.V(3).Info("enqueuing ...", "selector", selector.String())
	admrs, err := c.admrLister.List(selector)
	if err != nil {
		return err
	}
	for _, rcr := range admrs {
		controllerutils.Enqueue(logger, c.queue, rcr, controllerutils.MetaNamespaceKey)
	}
	cadmrs, err := c.cadmrLister.List(selector)
	if err != nil {
		return err
	}
	for _, crcr := range cadmrs {
		controllerutils.Enqueue(logger, c.queue, crcr, controllerutils.MetaNamespaceKey)
	}
	return nil
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.V(3).Info("reconciling ...")
	// try to find report from the cache
	report, err := reportutils.GetAdmissionReport(namespace, name, c.admrLister, c.cadmrLister)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// try to find resource from the cache
	resource, gvk, err := c.metadataCache.GetResource(reportutils.GetResourceUid(report))
	if err != nil {
		return err
	}
	// set owner if not done yet
	if resource != nil && len(report.GetOwnerReferences()) == 0 {
		reportutils.SetOwner(report, gvk.Group, gvk.Version, gvk.Kind, resource)
		_, err = reportutils.UpdateReport(report, c.client)
		return err
	}
	// cleanup old reports
	// if they are not the same version as the current resource version
	// and were created more than five minutes ago
	if resource == nil || resource.GetResourceVersion() != reportutils.GetResourceVersion(report) {
		if report.GetCreationTimestamp().Add(time.Minute * 5).Before(time.Now()) {
			return reportutils.DeleteReport(report, c.client)
		}
	}
	return nil
}
