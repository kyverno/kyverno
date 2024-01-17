package namespace

import (
	"context"
	"fmt"
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
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TODO: resync in resource controller
// TODO: policy hash

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
	return cache.ExplicitKey(obj.GetNamespace())
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

func (c *controller) mergeAdmissionReports(ctx context.Context, namespace string, policyMap map[string]policyMapEntry, accumulator map[string]policyreportv1alpha2.PolicyReportResult) error {
	if namespace == "" {
		next := ""
		for {
			cadms, err := c.client.KyvernoV1alpha2().ClusterAdmissionReports().List(ctx, metav1.ListOptions{
				// no need to consider non aggregated reports
				LabelSelector: reportutils.LabelAggregatedReport,
				Limit:         mergeLimit,
				Continue:      next,
			})
			if err != nil {
				return err
			}
			next = cadms.Continue
			for i := range cadms.Items {
				mergeReports(policyMap, accumulator, &cadms.Items[i])
			}
			if next == "" {
				return nil
			}
		}
	} else {
		next := ""
		for {
			adms, err := c.client.KyvernoV1alpha2().AdmissionReports(namespace).List(ctx, metav1.ListOptions{
				// no need to consider non aggregated reports
				LabelSelector: reportutils.LabelAggregatedReport,
				Limit:         mergeLimit,
				Continue:      next,
			})
			if err != nil {
				return err
			}
			next = adms.Continue
			for i := range adms.Items {
				mergeReports(policyMap, accumulator, &adms.Items[i])
			}
			if next == "" {
				return nil
			}
		}
	}
}

func (c *controller) mergeBackgroundScanReports(ctx context.Context, namespace string, policyMap map[string]policyMapEntry, accumulator map[string]policyreportv1alpha2.PolicyReportResult) error {
	if namespace == "" {
		next := ""
		for {
			cbgscans, err := c.client.KyvernoV1alpha2().ClusterBackgroundScanReports().List(ctx, metav1.ListOptions{
				Limit:    mergeLimit,
				Continue: next,
			})
			if err != nil {
				return err
			}
			next = cbgscans.Continue
			for i := range cbgscans.Items {
				mergeReports(policyMap, accumulator, &cbgscans.Items[i])
			}
			if next == "" {
				return nil
			}
		}
	} else {
		next := ""
		for {
			bgscans, err := c.client.KyvernoV1alpha2().BackgroundScanReports(namespace).List(ctx, metav1.ListOptions{
				Limit:    mergeLimit,
				Continue: next,
			})
			if err != nil {
				return err
			}
			next = bgscans.Continue
			for i := range bgscans.Items {
				mergeReports(policyMap, accumulator, &bgscans.Items[i])
			}
			if next == "" {
				return nil
			}
		}
	}
}

func (c *controller) reconcileReport(ctx context.Context, policyMap map[string]policyMapEntry, report kyvernov1alpha2.ReportInterface, namespace, name string, results ...policyreportv1alpha2.PolicyReportResult) (kyvernov1alpha2.ReportInterface, error) {
	if report == nil {
		report = reportutils.NewPolicyReport(namespace, name, nil, results...)
		for _, result := range results {
			policy := policyMap[result.Policy]
			if policy.policy != nil {
				reportutils.SetPolicyLabel(report, engineapi.NewKyvernoPolicy(policy.policy))
			}
		}
		return reportutils.CreateReport(ctx, report, c.client)
	}
	after := reportutils.DeepCopy(report)
	// hold custom labels
	reportutils.CleanupKyvernoLabels(after)
	reportutils.SetManagedByKyvernoLabel(after)
	for _, result := range results {
		policy := policyMap[result.Policy]
		if policy.policy != nil {
			reportutils.SetPolicyLabel(after, engineapi.NewKyvernoPolicy(policy.policy))
		}
	}
	reportutils.SetResults(after, results...)
	if datautils.DeepEqual(report, after) {
		return after, nil
	}
	return reportutils.UpdateReport(ctx, after, c.client)
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

func (c *controller) buildReportsResults(ctx context.Context, namespace string) ([]policyreportv1alpha2.PolicyReportResult, map[string]policyMapEntry, error) {
	policyMap, err := c.createPolicyMap()
	if err != nil {
		return nil, nil, err
	}
	merged := map[string]policyreportv1alpha2.PolicyReportResult{}
	if err := c.mergeAdmissionReports(ctx, namespace, policyMap, merged); err != nil {
		return nil, nil, err
	}
	if err := c.mergeBackgroundScanReports(ctx, namespace, policyMap, merged); err != nil {
		return nil, nil, err
	}
	var results []policyreportv1alpha2.PolicyReportResult
	for _, result := range merged {
		results = append(results, result)
	}
	return results, policyMap, nil
}

func (c *controller) getPolicyReports(ctx context.Context, namespace string) ([]kyvernov1alpha2.ReportInterface, error) {
	var reports []kyvernov1alpha2.ReportInterface
	if namespace == "" {
		list, err := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			if controllerutils.IsManagedByKyverno(&list.Items[i]) {
				reports = append(reports, &list.Items[i])
			}
		}
	} else {
		list, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			if controllerutils.IsManagedByKyverno(&list.Items[i]) {
				reports = append(reports, &list.Items[i])
			}
		}
	}
	return reports, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, _ string) error {
	results, policyMap, err := c.buildReportsResults(ctx, key)
	if err != nil {
		return err
	}
	policyReports, err := c.getPolicyReports(ctx, key)
	if err != nil {
		return err
	}
	actual := map[string]kyvernov1alpha2.ReportInterface{}
	for _, report := range policyReports {
		actual[report.GetName()] = report
	}
	splitReports := reportutils.SplitResultsByPolicy(logger, results)
	var expected []kyvernov1alpha2.ReportInterface
	chunkSize := c.chunkSize
	if chunkSize <= 0 {
		chunkSize = len(results)
	}
	for name, results := range splitReports {
		for i := 0; i < len(results); i += chunkSize {
			end := i + chunkSize
			if end > len(results) {
				end = len(results)
			}
			name := name
			if i > 0 {
				name = fmt.Sprintf("%s-%d", name, i/chunkSize)
			}
			report, err := c.reconcileReport(ctx, policyMap, actual[name], key, name, results[i:end]...)
			if err != nil {
				return err
			}
			expected = append(expected, report)
		}
	}
	return c.cleanReports(ctx, actual, expected)
}
