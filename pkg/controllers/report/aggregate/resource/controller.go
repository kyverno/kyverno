package resource

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "namespace-aggregate-report-controller"
	maxRetries     = 10
	mergeLimit     = 1000
	enqueueDelay   = 30 * time.Second
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
	polLister      kyvernov1listers.PolicyLister
	cpolLister     kyvernov1listers.ClusterPolicyLister
	admrLister     cache.GenericLister
	cadmrLister    cache.GenericLister
	bgscanrLister  cache.GenericLister
	cbgscanrLister cache.GenericLister

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

func keyFunc(obj metav1.Object) cache.ExplicitKey {
	return cache.ExplicitKey(obj.GetName())
}

func NewController(
	client versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	metadataCache resource.MetadataCache,
	chunkSize int,
) controllers.Controller {
	admrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	cadmrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	bgscanrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("backgroundscanreports"))
	cbgscanrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	polrInformer := metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("policyreports"))
	cpolrInformer := metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("clusterpolicyreports"))
	c := controller{
		client:         client,
		polLister:      polInformer.Lister(),
		cpolLister:     cpolInformer.Lister(),
		admrLister:     admrInformer.Lister(),
		cadmrLister:    cadmrInformer.Lister(),
		bgscanrLister:  bgscanrInformer.Lister(),
		cbgscanrLister: cbgscanrInformer.Lister(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		metadataCache:  metadataCache,
		chunkSize:      chunkSize,
	}
	if _, _, err := controllerutils.AddDelayedExplicitEventHandlers(logger, polrInformer.Informer(), c.queue, enqueueDelay, keyFunc); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedExplicitEventHandlers(logger, cpolrInformer.Informer(), c.queue, enqueueDelay, keyFunc); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedExplicitEventHandlers(logger, bgscanrInformer.Informer(), c.queue, enqueueDelay, keyFunc); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedExplicitEventHandlers(logger, cbgscanrInformer.Informer(), c.queue, enqueueDelay, keyFunc); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	enqueueFromAdmr := func(obj metav1.Object) {
		// no need to consider non aggregated reports
		if controllerutils.HasLabel(obj, reportutils.LabelAggregatedReport) {
			c.queue.AddAfter(keyFunc(obj), enqueueDelay)
		}
	}
	if _, err := controllerutils.AddEventHandlersT(
		admrInformer.Informer(),
		func(obj metav1.Object) { enqueueFromAdmr(obj) },
		func(_, obj metav1.Object) { enqueueFromAdmr(obj) },
		func(obj metav1.Object) { enqueueFromAdmr(obj) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		cadmrInformer.Informer(),
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

func (c *controller) cleanReports(ctx context.Context, actual map[string]kyvernov1alpha2.ReportInterface, expected []kyvernov1alpha2.ReportInterface) error {
	keep := sets.New[string]()
	for _, obj := range expected {
		keep.Insert(obj.GetName())
	}
	for _, obj := range actual {
		if !keep.Has(obj.GetName()) {
			err := reportutils.DeleteReport(ctx, obj, c.client)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeReports(policyMap map[string]policyMapEntry, accumulator map[string]policyreportv1alpha2.PolicyReportResult, reports ...kyvernov1alpha2.ReportInterface) {
	for _, report := range reports {
		if report != nil {
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
					currentPolicy := policyMap[result.Policy]
					if currentPolicy.rules != nil && currentPolicy.rules.Has(result.Rule) {
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
	}
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

func (c *controller) getBackgroundScanReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		report, err := c.client.KyvernoV1alpha2().ClusterBackgroundScanReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return report, nil
	} else {
		report, err := c.client.KyvernoV1alpha2().BackgroundScanReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return report, nil
	}
}

func (c *controller) getAdmissionReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		report, err := c.client.KyvernoV1alpha2().ClusterAdmissionReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return report, nil
	} else {
		report, err := c.client.KyvernoV1alpha2().AdmissionReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return report, nil
	}
}

func (c *controller) getPolicyReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		report, err := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return report, nil
	} else {
		report, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return report, nil
	}
}

func (c *controller) getReports(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, kyvernov1alpha2.ReportInterface, error) {
	admissionReport, err := c.getAdmissionReport(ctx, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}
	backgroundReport, err := c.getBackgroundScanReport(ctx, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}
	return admissionReport, backgroundReport, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, _ string) error {
	uid := types.UID(key)
	resource, gvk, exists := c.metadataCache.GetResourceHash(uid)
	if exists {
		admissionReport, backgroundReport, err := c.getReports(ctx, resource.Namespace, key)
		if err != nil {
			return err
		}
		if admissionReport == nil && backgroundReport == nil {
			return nil
		}
		policyReport, err := c.getPolicyReport(ctx, resource.Namespace, key)
		if err != nil {
			return err
		}
		create := false
		if policyReport == nil {
			create = true
			policyReport = reportutils.NewPolicyReport(resource.Namespace, key)
		}
		controllerutils.SetOwner(policyReport, gvk.GroupVersion().String(), gvk.Kind, resource.Name, uid)
		reportutils.SetResourceUid(policyReport, uid)
		// reportutils.SetResourceGVR(report, gvr)
		reportutils.SetResourceNamespaceAndName(policyReport, resource.Namespace, resource.Name)
		// aggregate reports
		policyMap, err := c.createPolicyMap()
		if err != nil {
			return err
		}
		merged := map[string]policyreportv1alpha2.PolicyReportResult{}
		mergeReports(policyMap, merged, policyReport, admissionReport, backgroundReport)
		var results []policyreportv1alpha2.PolicyReportResult
		for _, result := range merged {
			results = append(results, result)
		}
		if len(results) == 0 {
			if err := reportutils.DeleteReport(ctx, policyReport, c.client); err != nil {
				return err
			}
		} else {
			reportutils.SetResults(policyReport, results...)
			if create {
				if _, err := reportutils.CreateReport(ctx, policyReport, c.client); err != nil {
					return err
				}
			} else {
				if _, err := reportutils.UpdateReport(ctx, policyReport, c.client); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
