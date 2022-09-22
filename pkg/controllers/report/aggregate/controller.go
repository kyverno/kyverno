package aggregate

import (
	"reflect"
	"time"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	policyreportv1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
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
	polrLister     policyreportv1alpha2listers.PolicyReportLister
	cpolrLister    policyreportv1alpha2listers.ClusterPolicyReportLister
	admrLister     kyvernov1alpha2listers.AdmissionReportLister
	cadmrLister    kyvernov1alpha2listers.ClusterAdmissionReportLister
	bgscanrLister  kyvernov1alpha2listers.BackgroundScanReportLister
	cbgscanrLister kyvernov1alpha2listers.ClusterBackgroundScanReportLister

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
	polrInformer policyreportv1alpha2informers.PolicyReportInformer,
	cpolrInformer policyreportv1alpha2informers.ClusterPolicyReportInformer,
	admrInformer kyvernov1alpha2informers.AdmissionReportInformer,
	cadmrInformer kyvernov1alpha2informers.ClusterAdmissionReportInformer,
	bgscanrInformer kyvernov1alpha2informers.BackgroundScanReportInformer,
	cbgscanrInformer kyvernov1alpha2informers.ClusterBackgroundScanReportInformer,
	metadataCache resource.MetadataCache,
) *controller {
	c := controller{
		client:         client,
		polrLister:     polrInformer.Lister(),
		cpolrLister:    cpolrInformer.Lister(),
		admrLister:     admrInformer.Lister(),
		cadmrLister:    cadmrInformer.Lister(),
		bgscanrLister:  bgscanrInformer.Lister(),
		cbgscanrLister: cbgscanrInformer.Lister(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		metadataCache:  metadataCache,
	}
	controllerutils.AddExplicitEventHandlers(logger, polrInformer.Informer(), c.queue, keyFunc)
	controllerutils.AddExplicitEventHandlers(logger, cpolrInformer.Informer(), c.queue, keyFunc)
	controllerutils.AddExplicitEventHandlers(logger, admrInformer.Informer(), c.queue, keyFunc)
	controllerutils.AddExplicitEventHandlers(logger, cadmrInformer.Informer(), c.queue, keyFunc)
	controllerutils.AddExplicitEventHandlers(logger, bgscanrInformer.Informer(), c.queue, keyFunc)
	controllerutils.AddExplicitEventHandlers(logger, cbgscanrInformer.Informer(), c.queue, keyFunc)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) listReports(namespace string) ([]kyvernov1alpha2.ReportChangeRequestInterface, error) {
	var reports []kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		cadms, err := c.cadmrLister.List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, cadm := range cadms {
			reports = append(reports, cadm)
		}
		cbgscans, err := c.cbgscanrLister.List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, cbgscan := range cbgscans {
			reports = append(reports, cbgscan)
		}
	} else {
		adms, err := c.admrLister.AdmissionReports(namespace).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, adm := range adms {
			reports = append(reports, adm)
		}
		bgscans, err := c.bgscanrLister.BackgroundScanReports(namespace).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, bgscan := range bgscans {
			reports = append(reports, bgscan)
		}
	}
	return reports, nil
}

func (c *controller) reconcileReport(namespace, name string, results ...policyreportv1alpha2.PolicyReportResult) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	report, err := reportutils.GetPolicyReport(namespace, name, c.polrLister, c.cpolrLister)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reportutils.CreateReport(c.client, reportutils.NewPolicyReport(namespace, name, results...))
		}
		return nil, err
	}
	after := reportutils.DeepCopy(report)
	reportutils.SetResults(after, results...)
	if reflect.DeepEqual(report, after) {
		return after, nil
	}
	return reportutils.UpdateReport(after, c.client)
}

func (c *controller) reconcile(key, _, _ string) error {
	logger := logger.WithValues("key", key)
	logger.Info("reconciling ...")
	// delay processing to reduce reconciliation iterations
	// in case things are changing fast in the cluster
	time.Sleep(2 * time.Second)
	reports, err := c.listReports(key)
	if err != nil {
		return err
	}
	var results []policyreportv1alpha2.PolicyReportResult
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
				result.Resources = objectRefs
				results = append(results, result)
			}
		}
	}
	splitReports := reportutils.SplitResultsByPolicy(results)
	for name, results := range splitReports {
		_, err := c.reconcileReport(key, name, results...)
		if err != nil {
			return err
		}
	}
	return nil
}
