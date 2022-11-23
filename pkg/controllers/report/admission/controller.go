package admission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	queue workqueue.RateLimitingInterface

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
		metadataCache: metadataCache,
	}
	c.metadataCache.AddEventHandler(func(uid types.UID, _ schema.GroupVersionKind, _ resource.Resource) { queue.Add(cache.ExplicitKey(uid)) })
	controllerutils.AddEventHandlersT(
		admrInformer.Informer(),
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
		func(old, obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(old))) },
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
	)
	controllerutils.AddEventHandlersT(
		cadmrInformer.Informer(),
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
		func(old, obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(old))) },
		func(obj metav1.Object) { queue.Add(cache.ExplicitKey(reportutils.GetResourceUid(obj))) },
	)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) getReports(uid types.UID) ([]metav1.Object, error) {
	selector, err := reportutils.SelectorResourceUidEquals(uid)
	if err != nil {
		return nil, err
	}
	var results []metav1.Object
	admrs, err := c.admrLister.List(selector)
	if err != nil {
		return nil, err
	}
	for _, admr := range admrs {
		results = append(results, admr.(metav1.Object))
	}
	cadmrs, err := c.cadmrLister.List(selector)
	if err != nil {
		return nil, err
	}
	for _, cadmr := range cadmrs {
		results = append(results, cadmr.(metav1.Object))
	}
	return results, nil
}

func (c *controller) getReportMetadata(namespace, name string) (metav1.Object, error) {
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

func mergeReports(accumulator map[string]policyreportv1alpha2.PolicyReportResult, reports ...kyvernov1alpha2.ReportInterface) {
	for _, report := range reports {
		if len(report.GetOwnerReferences()) == 1 {
			ownerRef := report.GetOwnerReferences()[0]
			objectRefs := []corev1.ObjectReference{{
				APIVersion: ownerRef.APIVersion,
				Kind:       ownerRef.Kind,
				Namespace:  report.GetNamespace(),
				Name:       ownerRef.Name,
				UID:        ownerRef.UID,
			}}
			for _, result := range report.GetResults() {
				key := result.Policy + "/" + result.Rule + "/" + string(ownerRef.UID)
				result.Resources = objectRefs
				if rule, exists := accumulator[key]; !exists {
					accumulator[key] = result
				} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
					accumulator[key] = result
				}
			}
		}
	}
}

func (c *controller) aggregateReports(ctx context.Context, uid types.UID, gvk schema.GroupVersionKind, res resource.Resource, reports ...metav1.Object) error {
	merged := map[string]policyreportv1alpha2.PolicyReportResult{}
	var toDelete []metav1.Object
	for _, report := range reports {
		if reportutils.GetResourceHash(report) == res.Hash {
			// TODO: see if we can use List instead of fetching reports one by one
			report, err := c.getReport(ctx, report.GetNamespace(), report.GetName())
			if err != nil {
				return err
			}
			mergeReports(merged, report)
		}
		if report.GetName() != string(uid) {
			if reportutils.GetResourceHash(report) == res.Hash || report.GetCreationTimestamp().Add(time.Minute*2).Before(time.Now()) {
				toDelete = append(toDelete, report)
			} else {
				c.queue.AddAfter(cache.ExplicitKey(uid), time.Minute*2)
			}
		}
	}
	var results []policyreportv1alpha2.PolicyReportResult
	for _, result := range merged {
		results = append(results, result)
	}
	before, err := c.getReport(ctx, res.Namespace, string(uid))
	if err != nil {
		if apierrors.IsNotFound(err) {
			if len(results) == 0 {
				return nil
			}
			report := reportutils.EmptyAdmissionReport(uid, res.Namespace, res.Name, metav1.GroupVersionKind(gvk))
			controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, res.Name, uid)
			controllerutils.SetLabel(report, reportutils.LabelResourceHash, res.Hash)
			controllerutils.SetLabel(report, reportutils.LabelAggregatedReport, res.Hash)
			reportutils.SetResults(report, results...)
			_, err := reportutils.CreateReport(ctx, report, c.client)
			return err
		}
		return err
	}
	after := reportutils.DeepCopy(before)
	controllerutils.SetLabel(after, reportutils.LabelResourceHash, res.Hash)
	controllerutils.SetLabel(after, reportutils.LabelAggregatedReport, res.Hash)
	reportutils.SetResults(after, results...)
	if !utils.ReportsAreIdentical(before, after) {
		_, err = reportutils.UpdateReport(ctx, after, c.client)
		if err != nil {
			return err
		}
	}
	var errs []error
	for _, report := range toDelete {
		if err := c.deleteReport(ctx, report.GetNamespace(), report.GetName()); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, _ string) error {
	resourceUid := types.UID(key)
	// find related reports
	reports, err := c.getReports(resourceUid)
	if err != nil {
		return err
	} else {
		var toDelete []metav1.Object
		for _, report := range reports {
			if report.GetName() != string(resourceUid) {
				if report.GetCreationTimestamp().Add(time.Minute * 2).Before(time.Now()) {
					toDelete = append(toDelete, report)
				}
			}
		}
		var errs []error
		for _, report := range toDelete {
			if err := c.deleteReport(ctx, report.GetNamespace(), report.GetName()); err != nil {
				errs = append(errs, err)
			}
		}
		if err := multierr.Combine(errs...); err != nil {
			return err
		}
	}
	// is the resource known
	resource, gvk, found := c.metadataCache.GetResourceHash(resourceUid)
	if !found {
		return nil
	}
	// set orphan reports an owner
	for _, report := range reports {
		if len(report.GetOwnerReferences()) == 0 {
			report, err := c.getReport(ctx, report.GetNamespace(), report.GetName())
			if err != nil {
				return err
			}
			controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, resource.Name, resourceUid)
			_, err = reportutils.UpdateReport(ctx, report, c.client)
			return err
		}
	}
	// build an aggregated report
	return c.aggregateReports(ctx, resourceUid, gvk, resource, reports...)
}
