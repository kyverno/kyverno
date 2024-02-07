package aggregate

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 10
	ControllerName = "aggregate-report-controller"
	maxRetries     = 10
	enqueueDelay   = 10 * time.Second
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	vapLister  admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyLister

	// queue
	queue workqueue.RateLimitingInterface

	// cache
	metadataCache resource.MetadataCache

	chunkSize int
}

type policyMapEntry struct {
	policy kyvernov1.PolicyInterface
	rules  sets.Set[string]
}

func NewController(
	client versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vapInformer admissionregistrationv1alpha1informers.ValidatingAdmissionPolicyInformer,
	metadataCache resource.MetadataCache,
	chunkSize int,
) controllers.Controller {
	ephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))
	polrInformer := metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("policyreports"))
	cpolrInformer := metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("clusterpolicyreports"))
	c := controller{
		client:        client,
		polLister:     polInformer.Lister(),
		cpolLister:    cpolInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		metadataCache: metadataCache,
		chunkSize:     chunkSize,
	}
	enqueueAll := func() {
		if list, err := polrInformer.Lister().List(labels.Everything()); err == nil {
			for _, item := range list {
				c.queue.AddAfter(controllerutils.MetaObjectToName(item.(*metav1.PartialObjectMetadata)), enqueueDelay)
			}
		}
		if list, err := cpolrInformer.Lister().List(labels.Everything()); err == nil {
			for _, item := range list {
				c.queue.AddAfter(controllerutils.MetaObjectToName(item.(*metav1.PartialObjectMetadata)), enqueueDelay)
			}
		}
	}
	if _, err := controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		func(_ metav1.Object) { enqueueAll() },
		func(_, _ metav1.Object) { enqueueAll() },
		func(_ metav1.Object) { enqueueAll() },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		cpolInformer.Informer(),
		func(_ metav1.Object) { enqueueAll() },
		func(_, _ metav1.Object) { enqueueAll() },
		func(_ metav1.Object) { enqueueAll() },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if vapInformer != nil {
		c.vapLister = vapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			vapInformer.Informer(),
			func(_ metav1.Object) { enqueueAll() },
			func(_, _ metav1.Object) { enqueueAll() },
			func(_ metav1.Object) { enqueueAll() },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	metadataCache.AddEventHandler(func(eventType resource.EventType, uid types.UID, _ schema.GroupVersionKind, res resource.Resource) {
		// if it's a deletion, nothing to do
		if eventType == resource.Deleted {
			return
		}
		if res.Namespace == "" {
			c.queue.AddAfter(string(uid), enqueueDelay)
		} else {
			c.queue.AddAfter(res.Namespace+"/"+string(uid), enqueueDelay)
		}
	})
	enqueueFromAdmr := func(obj metav1.Object) {
		switch reportutils.GetSource(obj) {
		case "background-scan":
			c.queue.AddAfter(controllerutils.MetaObjectToName(obj), enqueueDelay)
		case "admission":
			obj := cache.ObjectName{Namespace: obj.GetNamespace(), Name: string(reportutils.GetResourceUid(obj))}
			c.queue.AddAfter(obj.String(), enqueueDelay)
		}
	}
	if _, err := controllerutils.AddEventHandlersT(
		ephrInformer.Informer(),
		func(obj metav1.Object) { enqueueFromAdmr(obj) },
		func(_, obj metav1.Object) { enqueueFromAdmr(obj) },
		func(obj metav1.Object) { enqueueFromAdmr(obj) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		cephrInformer.Informer(),
		func(obj metav1.Object) { enqueueFromAdmr(obj) },
		func(_, obj metav1.Object) { enqueueFromAdmr(obj) },
		func(obj metav1.Object) { enqueueFromAdmr(obj) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) createPolicyMap() (map[string]policyMapEntry, error) {
	results := map[string]policyMapEntry{}
	cpols, err := c.cpolLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, cpol := range cpols {
		key, err := cache.MetaNamespaceKeyFunc(cpol)
		if err != nil {
			return nil, err
		}
		results[key] = policyMapEntry{
			policy: cpol,
			rules:  sets.New[string](),
		}
		for _, rule := range autogen.ComputeRules(cpol) {
			results[key].rules.Insert(rule.Name)
		}
	}
	pols, err := c.polLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, pol := range pols {
		key, err := cache.MetaNamespaceKeyFunc(pol)
		if err != nil {
			return nil, err
		}
		results[key] = policyMapEntry{
			policy: pol,
			rules:  sets.New[string](),
		}
		for _, rule := range autogen.ComputeRules(pol) {
			results[key].rules.Insert(rule.Name)
		}
	}
	return results, nil
}

func (c *controller) createVapMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.vapLister != nil {
		vaps, err := c.vapLister.List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, vap := range vaps {
			key, err := cache.MetaNamespaceKeyFunc(vap)
			if err != nil {
				return nil, err
			}
			results.Insert(key)
		}
	}
	return results, nil
}

func (c *controller) getBackgroundScanReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		report, err := c.client.ReportsV1().ClusterEphemeralReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return report, nil
	} else {
		report, err := c.client.ReportsV1().EphemeralReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return report, nil
	}
}

func (c *controller) getAdmissionReports(ctx context.Context, namespace, name string) ([]kyvernov1alpha2.ReportInterface, error) {
	selector, err := reportutils.SelectorResourceUidEquals(types.UID(name))
	if err != nil {
		return nil, err
	}
	var results []kyvernov1alpha2.ReportInterface
	if namespace == "" {
		reports, err := c.client.ReportsV1().ClusterEphemeralReports().List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, report := range reports.Items {
			report := report
			results = append(results, &report)
		}
	} else {
		reports, err := c.client.ReportsV1().EphemeralReports(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, report := range reports.Items {
			report := report
			results = append(results, &report)
		}
	}
	return results, nil
}

func (c *controller) getPolicyReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		report, err := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return report, nil
	} else {
		report, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return report, nil
	}
}

func (c *controller) getReports(ctx context.Context, namespace, name string) ([]kyvernov1alpha2.ReportInterface, kyvernov1alpha2.ReportInterface, error) {
	admissionReports, err := c.getAdmissionReports(ctx, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	backgroundReport, err := c.getBackgroundScanReport(ctx, namespace, name)
	if err != nil {
		return nil, nil, err
	}
	return admissionReports, backgroundReport, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, _, namespace, name string) error {
	uid := types.UID(name)
	resource, gvk, exists := c.metadataCache.GetResourceHash(uid)
	if exists {
		admissionReports, backgroundReport, err := c.getReports(ctx, namespace, name)
		if err != nil {
			return err
		}
		policyReport, err := c.getPolicyReport(ctx, namespace, name)
		if err != nil {
			return err
		}
		create := false
		scope := &corev1.ObjectReference{
			Kind:       gvk.Kind,
			Namespace:  namespace,
			Name:       resource.Name,
			UID:        uid,
			APIVersion: gvk.GroupVersion().String(),
		}
		if policyReport == nil {
			create = true
			policyReport = reportutils.NewPolicyReport(namespace, name, scope)
			controllerutils.SetOwner(policyReport, gvk.GroupVersion().String(), gvk.Kind, resource.Name, uid)
		}
		// aggregate reports
		policyMap, err := c.createPolicyMap()
		if err != nil {
			return err
		}
		vapMap, err := c.createVapMap()
		if err != nil {
			return err
		}
		merged := map[string]policyreportv1alpha2.PolicyReportResult{}
		var reports []kyvernov1alpha2.ReportInterface
		reports = append(reports, policyReport)
		reports = append(reports, backgroundReport)
		reports = append(reports, admissionReports...)
		mergeReports(policyMap, vapMap, merged, uid, reports...)
		var results []policyreportv1alpha2.PolicyReportResult
		for _, result := range merged {
			results = append(results, result)
		}
		if len(results) == 0 {
			if !create {
				if err := deleteReport(ctx, policyReport, c.client); err != nil {
					return err
				}
			}
		} else {
			reportutils.SetResults(policyReport, results...)
			if create {
				if _, err := reportutils.CreateReport(ctx, policyReport, c.client); err != nil {
					return err
				}
			} else {
				if _, err := updateReport(ctx, policyReport, c.client); err != nil {
					return err
				}
			}
		}
		for _, admissionReport := range admissionReports {
			if err := deleteReport(ctx, admissionReport, c.client); err != nil {
				return err
			}
		}
		if backgroundReport != nil {
			if err := deleteReport(ctx, backgroundReport, c.client); err != nil {
				return err
			}
		}
	} else {
		policyReport, err := c.getPolicyReport(ctx, namespace, name)
		if err != nil {
			return err
		}
		if policyReport == nil {
			return nil
		}
		admissionReports, backgroundReport, err := c.getReports(ctx, namespace, name)
		if err != nil {
			return err
		}
		// aggregate reports
		policyMap, err := c.createPolicyMap()
		if err != nil {
			return err
		}
		vapMap, err := c.createVapMap()
		if err != nil {
			return err
		}
		merged := map[string]policyreportv1alpha2.PolicyReportResult{}
		var reports []kyvernov1alpha2.ReportInterface
		reports = append(reports, policyReport)
		reports = append(reports, backgroundReport)
		reports = append(reports, admissionReports...)
		mergeReports(policyMap, vapMap, merged, uid, reports...)
		var results []policyreportv1alpha2.PolicyReportResult
		for _, result := range merged {
			results = append(results, result)
		}
		if len(results) == 0 {
			if err := deleteReport(ctx, policyReport, c.client); err != nil {
				return err
			}
		}
	}
	return nil
}
