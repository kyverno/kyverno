package report

import (
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	policyreportv1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	auditcontroller "github.com/kyverno/kyverno/pkg/controllers/audit"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 5
)

// TODO: improve merging

type controller struct {
	// clients
	client versioned.Interface

	// listers
	polrLister  policyreportv1alpha2listers.PolicyReportLister
	cpolrLister policyreportv1alpha2listers.ClusterPolicyReportLister
	rcrLister   kyvernov1alpha2listers.ReportChangeRequestLister
	crcrLister  kyvernov1alpha2listers.ClusterReportChangeRequestLister

	// queue
	queue workqueue.RateLimitingInterface
}

func keyFunc(obj metav1.Object) cache.ExplicitKey {
	return cache.ExplicitKey(obj.GetNamespace())
}

func NewController(
	client versioned.Interface,
	polrInformer policyreportv1alpha2informers.PolicyReportInformer,
	cpolrInformer policyreportv1alpha2informers.ClusterPolicyReportInformer,
	rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
) *controller {
	c := controller{
		client:      client,
		polrLister:  polrInformer.Lister(),
		cpolrLister: cpolrInformer.Lister(),
		rcrLister:   rcrInformer.Lister(),
		crcrLister:  crcrInformer.Lister(),
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddExplicitEventHandlers(logger, rcrInformer.Informer(), c.queue, keyFunc)
	controllerutils.AddExplicitEventHandlers(logger, crcrInformer.Informer(), c.queue, keyFunc)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) reconcile(key, _, _ string) error {
	logger := logger.WithValues("key", key)
	logger.Info("reconciling ...")
	// delay processing to reduce reconciliation iterations
	// in case things are changing fast in the cluster
	time.Sleep(2 * time.Second)
	if key == "" {
		return c.reconcileClusterReport()
	} else {
		return c.reconcileReport(key)
	}
}

func (c *controller) reconcileClusterReport() error {
	lister := c.cpolrLister
	client := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports()
	crcrs, err := c.crcrLister.List(labels.Everything())
	if err != nil {
		return err
	}
	var results []policyreportv1alpha2.PolicyReportResult
	for _, crcr := range crcrs {
		results = append(results, crcr.Results...)
	}
	splitResults := auditcontroller.SplitResultsByPolicy(results)
	var expected []*policyreportv1alpha2.ClusterPolicyReport
	for name := range splitResults {
		obj, err := controllerutils.CreateOrUpdate(name, lister, client,
			func(obj *policyreportv1alpha2.ClusterPolicyReport) error {
				controllerutils.SetLabel(obj, kyvernov1.LabelAppManagedBy, kyvernov1.ValueKyvernoApp)
				obj.Results = splitResults[name]
				obj.Summary = auditcontroller.CalculateSummary(splitResults[name])
				return nil
			},
		)
		if err != nil {
			return err
		}
		expected = append(expected, obj)
	}
	actual, err := lister.List(labels.Everything())
	if err != nil {
		return err
	}
	return controllerutils.Cleanup(actual, expected, client)
}

func (c *controller) reconcileReport(namespace string) error {
	lister := c.polrLister.PolicyReports(namespace)
	client := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace)
	rcrs, err := c.rcrLister.ReportChangeRequests(namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	var results []policyreportv1alpha2.PolicyReportResult
	for _, rcr := range rcrs {
		results = append(results, rcr.Results...)
	}
	splitResults := auditcontroller.SplitResultsByPolicy(results)
	var expected []*policyreportv1alpha2.PolicyReport
	for name := range splitResults {
		obj, err := controllerutils.CreateOrUpdate(name, lister, client,
			func(obj *policyreportv1alpha2.PolicyReport) error {
				obj.SetNamespace(namespace)
				controllerutils.SetLabel(obj, kyvernov1.LabelAppManagedBy, kyvernov1.ValueKyvernoApp)
				obj.Results = splitResults[name]
				obj.Summary = auditcontroller.CalculateSummary(splitResults[name])
				return nil
			},
		)
		if err != nil {
			return err
		}
		expected = append(expected, obj)
	}
	actual, err := lister.List(labels.Everything())
	if err != nil {
		return err
	}
	return controllerutils.Cleanup(actual, expected, client)
}
