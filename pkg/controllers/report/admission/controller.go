package admission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/controllers"
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
	// Workers is the number of workers for this controller
	Workers        = 2
	ControllerName = "admission-report-controller"
	maxRetries     = 10
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
) controllers.Controller {
	admrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	cadmrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		client:        client,
		admrLister:    admrInformer.Lister(),
		cadmrLister:   cadmrInformer.Lister(),
		queue:         queue,
		admrEnqueue:   controllerutils.AddDefaultEventHandlers(logger, admrInformer.Informer(), queue),
		cadmrEnqueue:  controllerutils.AddDefaultEventHandlers(logger, cadmrInformer.Informer(), queue),
		metadataCache: metadataCache,
	}
	c.metadataCache.AddEventHandler(func(uid types.UID, _ schema.GroupVersionKind, _ resource.Resource) {
		selector, err := reportutils.SelectorResourceUidEquals(uid)
		if err != nil {
			logger.Error(err, "failed to create label selector")
		}
		if err := c.enqueue(selector); err != nil {
			logger.Error(err, "failed to enqueue")
		}
	})
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) enqueue(selector labels.Selector) error {
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

func (c *controller) deleteReport(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		return c.client.KyvernoV1alpha2().ClusterAdmissionReports().Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		return c.client.KyvernoV1alpha2().AdmissionReports(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
}

func (c *controller) getReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		return c.client.KyvernoV1alpha2().ClusterAdmissionReports().Get(ctx, name, metav1.GetOptions{})
	} else {
		return c.client.KyvernoV1alpha2().AdmissionReports(namespace).Get(ctx, name, metav1.GetOptions{})
	}
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, namespace, name string) error {
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
	resource, gvk, found := c.metadataCache.GetResourceHash(uid)
	// set owner if not done yet
	if found && len(meta.GetOwnerReferences()) == 0 {
		report, err := c.getReport(ctx, namespace, name)
		if err != nil {
			return err
		}
		controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, resource.Name, uid)
		_, err = reportutils.UpdateReport(ctx, report, c.client)
		return err
	}
	// cleanup old reports
	// if they are not the same version as the current resource version
	// and were created more than 2 minutes ago
	if !found {
		// if we didn't find the resource, either no policy exist for this kind
		// or the resource was never created, we delete the report if it has no owner
		// and was created more than 2 minutes ago
		if len(meta.GetOwnerReferences()) == 0 && meta.GetCreationTimestamp().Add(time.Minute*2).Before(time.Now()) {
			return c.deleteReport(ctx, namespace, name)
		}
	} else {
		// if hashes don't match and the report was created more than 2
		// minutes ago we consider it obsolete and delete the report
		if !reportutils.CompareHash(meta, resource.Hash) && meta.GetCreationTimestamp().Add(time.Minute*2).Before(time.Now()) {
			return c.deleteReport(ctx, namespace, name)
		}
	}
	return nil
}
