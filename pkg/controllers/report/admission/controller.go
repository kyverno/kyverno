package admission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 2
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
	admrLister  cache.GenericLister
	cadmrLister cache.GenericLister

	// queue
	queue        workqueue.RateLimitingInterface
	admrEnqueue  controllerutils.EnqueueFunc
	cadmrEnqueue controllerutils.EnqueueFunc

	// cache
	metadataCache resource.MetadataCache
}

func NewController(
	client versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	metadataCache resource.MetadataCache,
) *controller {
	admrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	cadmrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)
	c := controller{
		client:        client,
		admrLister:    admrInformer.Lister(),
		cadmrLister:   cadmrInformer.Lister(),
		queue:         queue,
		admrEnqueue:   controllerutils.AddDefaultEventHandlers(logger.V(3), admrInformer.Informer(), queue),
		cadmrEnqueue:  controllerutils.AddDefaultEventHandlers(logger.V(3), cadmrInformer.Informer(), queue),
		metadataCache: metadataCache,
	}
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	c.metadataCache.AddEventHandler(func(uid types.UID, _ schema.GroupVersionKind, _ resource.Resource) {
		selector, err := reportutils.SelectorResourceUidEquals(uid)
		if err != nil {
			logger.Error(err, "failed to create label selector")
		}
		if err := c.enqueue(selector); err != nil {
			logger.Error(err, "failed to enqueue")
		}
	})
	controllerutils.Run(controllerName, logger.V(3), c.queue, workers, maxRetries, c.reconcile, stopCh)
}

func (c *controller) enqueue(selector labels.Selector) error {
	logger.V(3).Info("enqueuing ...", "selector", selector.String())
	admrs, err := c.admrLister.List(selector)
	if err != nil {
		return err
	}
	for _, adm := range admrs {
		err = c.admrEnqueue(adm)
		if err != nil {
			logger.Error(err, "failed to enqueue")
		}
	}
	cadmrs, err := c.cadmrLister.List(selector)
	if err != nil {
		return err
	}
	for _, cadmr := range cadmrs {
		err = c.admrEnqueue(cadmr)
		if err != nil {
			logger.Error(err, "failed to enqueue")
		}
	}
	return nil
}

func (c *controller) getMeta(namespace, name string) (metav1.Object, error) {
	if namespace == "" {
		obj, err := c.cadmrLister.Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(metav1.Object), err
	} else {
		obj, err := c.admrLister.ByNamespace(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(metav1.Object), err
	}
}

func (c *controller) deleteReport(namespace, name string) error {
	if namespace == "" {
		return c.client.KyvernoV1alpha2().ClusterAdmissionReports().Delete(context.TODO(), name, metav1.DeleteOptions{})
	} else {
		return c.client.KyvernoV1alpha2().AdmissionReports(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
}

func (c *controller) getReport(namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		return c.client.KyvernoV1alpha2().ClusterAdmissionReports().Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		return c.client.KyvernoV1alpha2().AdmissionReports(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	}
}

func (c *controller) reconcile(logger logr.Logger, key, namespace, name string) error {
	// try to find meta from the cache
	meta, err := c.getMeta(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// try to find resource from the cache
	uid := reportutils.GetResourceUid(meta)
	resource, gvk, exists := c.metadataCache.GetResourceHash(uid)
	// set owner if not done yet
	if exists && len(meta.GetOwnerReferences()) == 0 {
		report, err := c.getReport(namespace, name)
		if err != nil {
			return err
		}
		controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, resource.Name, uid)
		_, err = reportutils.UpdateReport(report, c.client)
		return err
	}
	// cleanup old reports
	// if they are not the same version as the current resource version
	// and were created more than five minutes ago
	if !exists || !reportutils.CompareHash(meta, resource.Hash) {
		if meta.GetCreationTimestamp().Add(time.Minute * 5).Before(time.Now()) {
			return c.deleteReport(namespace, name)
		}
	}
	return nil
}
