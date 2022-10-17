package aggregate

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// TODO: resync in resource controller
// TODO: error handling in resource controller
// TODO: policy hash

const (
	// Workers is the number of workers for this controller
	Workers        = 1
	ControllerName = "aggregate-report-controller"
	maxRetries     = 10
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
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

func keyFunc(obj metav1.Object) cache.ExplicitKey {
	return cache.ExplicitKey(obj.GetNamespace())
}

func NewController(
	client versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	metadataCache resource.MetadataCache,
	chunkSize int,
) controllers.Controller {
	admrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	cadmrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	bgscanrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("backgroundscanreports"))
	cbgscanrInformer := metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	c := controller{
		client:         client,
		admrLister:     admrInformer.Lister(),
		cadmrLister:    cadmrInformer.Lister(),
		bgscanrLister:  bgscanrInformer.Lister(),
		cbgscanrLister: cbgscanrInformer.Lister(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		metadataCache:  metadataCache,
		chunkSize:      chunkSize,
	}
	delay := 15 * time.Second
	controllerutils.AddDelayedExplicitEventHandlers(logger.V(3), admrInformer.Informer(), c.queue, delay, keyFunc)
	controllerutils.AddDelayedExplicitEventHandlers(logger.V(3), cadmrInformer.Informer(), c.queue, delay, keyFunc)
	controllerutils.AddDelayedExplicitEventHandlers(logger.V(3), bgscanrInformer.Informer(), c.queue, delay, keyFunc)
	controllerutils.AddDelayedExplicitEventHandlers(logger.V(3), cbgscanrInformer.Informer(), c.queue, delay, keyFunc)
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	controllerutils.Run(ctx, logger.V(3), ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) listAdmissionReports(ctx context.Context, namespace string) ([]kyvernov1alpha2.ReportInterface, error) {
	var reports []kyvernov1alpha2.ReportInterface
	if namespace == "" {
		cadms, err := c.client.KyvernoV1alpha2().ClusterAdmissionReports().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range cadms.Items {
			reports = append(reports, &cadms.Items[i])
		}
	} else {
		adms, err := c.client.KyvernoV1alpha2().AdmissionReports(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range adms.Items {
			reports = append(reports, &adms.Items[i])
		}
	}
	return reports, nil
}

func (c *controller) listBackgroundScanReports(ctx context.Context, namespace string) ([]kyvernov1alpha2.ReportInterface, error) {
	var reports []kyvernov1alpha2.ReportInterface
	if namespace == "" {
		cbgscans, err := c.client.KyvernoV1alpha2().ClusterBackgroundScanReports().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range cbgscans.Items {
			reports = append(reports, &cbgscans.Items[i])
		}
	} else {
		bgscans, err := c.client.KyvernoV1alpha2().BackgroundScanReports(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range bgscans.Items {
			reports = append(reports, &bgscans.Items[i])
		}
	}
	return reports, nil
}

func (c *controller) reconcileReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, namespace, name string, results ...policyreportv1alpha2.PolicyReportResult) (kyvernov1alpha2.ReportInterface, error) {
	if report == nil {
		return reportutils.CreateReport(ctx, reportutils.NewPolicyReport(namespace, name, results...), c.client)
	}
	after := reportutils.DeepCopy(report)
	reportutils.SetResults(after, results...)
	if reflect.DeepEqual(report, after) {
		return after, nil
	}
	return reportutils.UpdateReport(ctx, after, c.client)
}

func (c *controller) cleanReports(ctx context.Context, actual map[string]kyvernov1alpha2.ReportInterface, expected []kyvernov1alpha2.ReportInterface) error {
	keep := sets.NewString()
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

func (c *controller) buildReportsResults(ctx context.Context, namepsace string) ([]policyreportv1alpha2.PolicyReportResult, error) {
	merged := map[string]policyreportv1alpha2.PolicyReportResult{}
	{
		reports, err := c.listAdmissionReports(ctx, namepsace)
		if err != nil {
			return nil, err
		}
		mergeReports(merged, reports...)
	}
	{
		reports, err := c.listBackgroundScanReports(ctx, namepsace)
		if err != nil {
			return nil, err
		}
		mergeReports(merged, reports...)
	}
	var results []policyreportv1alpha2.PolicyReportResult
	for _, result := range merged {
		results = append(results, result)
	}
	return results, nil
}

func (c *controller) getPolicyReports(ctx context.Context, namespace string) ([]kyvernov1alpha2.ReportInterface, error) {
	var reports []kyvernov1alpha2.ReportInterface
	if namespace == "" {
		list, err := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			reports = append(reports, &list.Items[i])
		}
	} else {
		list, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			reports = append(reports, &list.Items[i])
		}
	}
	return reports, nil
}

func (c *controller) reconcile(ctx context.Context, logger logr.Logger, key, _, _ string) error {
	results, err := c.buildReportsResults(ctx, key)
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
			report, err := c.reconcileReport(ctx, actual[name], key, name, results[i:end]...)
			if err != nil {
				return err
			}
			expected = append(expected, report)
		}
	}
	return c.cleanReports(ctx, actual, expected)
}
