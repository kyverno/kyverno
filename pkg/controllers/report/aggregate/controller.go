package aggregate

import (
	"context"
	"fmt"
	"reflect"
	"time"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
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

const (
	maxRetries = 10
	workers    = 1
	chunkSize  = 1000
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
}

func keyFunc(obj metav1.Object) cache.ExplicitKey {
	return cache.ExplicitKey(obj.GetNamespace())
}

func NewController(
	client versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	metadataCache resource.MetadataCache,
) *controller {
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
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		metadataCache:  metadataCache,
	}
	delay := 15 * time.Second
	controllerutils.AddDelayedExplicitEventHandlers(logger, admrInformer.Informer(), c.queue, delay, keyFunc)
	controllerutils.AddDelayedExplicitEventHandlers(logger, cadmrInformer.Informer(), c.queue, delay, keyFunc)
	controllerutils.AddDelayedExplicitEventHandlers(logger, bgscanrInformer.Informer(), c.queue, delay, keyFunc)
	controllerutils.AddDelayedExplicitEventHandlers(logger, cbgscanrInformer.Informer(), c.queue, delay, keyFunc)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) listReports(namespace string) ([]kyvernov1alpha2.ReportChangeRequestInterface, error) {
	var reports []kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		cadms, err := c.client.KyvernoV1alpha2().ClusterAdmissionReports().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range cadms.Items {
			reports = append(reports, &cadms.Items[i])
		}
		cbgscans, err := c.client.KyvernoV1alpha2().ClusterBackgroundScanReports().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range cbgscans.Items {
			reports = append(reports, &cbgscans.Items[i])
		}
	} else {
		adms, err := c.client.KyvernoV1alpha2().AdmissionReports(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range adms.Items {
			reports = append(reports, &adms.Items[i])
		}
		bgscans, err := c.client.KyvernoV1alpha2().BackgroundScanReports(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range bgscans.Items {
			reports = append(reports, &bgscans.Items[i])
		}
	}
	return reports, nil
}

func (c *controller) reconcileReport(report kyvernov1alpha2.ReportChangeRequestInterface, namespace, name string, results ...policyreportv1alpha2.PolicyReportResult) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	if report == nil {
		return reportutils.CreateReport(c.client, reportutils.NewPolicyReport(namespace, name, results...))
	}
	after := reportutils.DeepCopy(report)
	reportutils.SetResults(after, results...)
	if reflect.DeepEqual(report, after) {
		return after, nil
	}
	return reportutils.UpdateReport(after, c.client)
}

func (c *controller) cleanReports(actual map[string]kyvernov1alpha2.ReportChangeRequestInterface, expected []kyvernov1alpha2.ReportChangeRequestInterface) error {
	keep := sets.NewString()
	for _, obj := range expected {
		keep.Insert(obj.GetName())
	}
	for _, obj := range actual {
		if !keep.Has(obj.GetName()) {
			err := reportutils.DeleteReport(obj, c.client)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *controller) reconcile(key, _, _ string) error {
	logger := logger.WithValues("key", key)
	logger.Info("reconciling ...")
	// delay processing to reduce reconciliation iterations
	// in case things are changing fast in the cluster
	reports, err := c.listReports(key)
	if err != nil {
		return err
	}
	merged := map[string]policyreportv1alpha2.PolicyReportResult{}
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
				if rule, exists := merged[key]; !exists {
					merged[key] = result
				} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
					merged[key] = result
				}
			}
		}
	}
	var results []policyreportv1alpha2.PolicyReportResult
	for _, result := range merged {
		results = append(results, result)
	}
	policyReports, err := reportutils.GetPolicyReports(key, c.client.Wgpolicyk8sV1alpha2())
	if err != nil {
		return err
	}
	actual := map[string]kyvernov1alpha2.ReportChangeRequestInterface{}
	for _, report := range policyReports {
		actual[report.GetName()] = report
	}
	splitReports := reportutils.SplitResultsByPolicy(results)
	var expected []kyvernov1alpha2.ReportChangeRequestInterface
	for name, results := range splitReports {
		for i := 0; i < len(results); i += chunkSize {
			end := i + chunkSize
			if end > len(results) {
				end = len(results)
			}
			name := fmt.Sprintf("%s-%d", name, i/1000)
			report, err := c.reconcileReport(actual[name], key, name, results[i:end]...)
			if err != nil {
				return err
			}
			expected = append(expected, report)
		}
	}
	return c.cleanReports(actual, expected)
}
