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
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
	cpolrName  = "cluster"
)

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

// TODO: split reports
// TODO: aggregate results

// DONE: cpol aggregation
// DONE: managed by kyverno label
// DONE: deep copy if coming from cache
// DONE: controllerutils.CreateOrUpdate
// DONE: controllerutils.GetOrNew

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
	controllerutils.AddKeyedEventHandlers(logger, rcrInformer.Informer(), c.queue, controllerutils.Explicit(keyFunc))
	controllerutils.AddKeyedEventHandlers(logger, crcrInformer.Informer(), c.queue, controllerutils.Explicit(keyFunc))
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
		return c.rebuildClusterReport()
	} else {
		return c.rebuildReport(key)
	}
}

func (c *controller) rebuildClusterReport() error {
	_, err := controllerutils.CreateOrUpdate(
		cpolrName,
		c.cpolrLister,
		c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports(),
		func(obj *policyreportv1alpha2.ClusterPolicyReport) error {
			controllerutils.SetLabel(obj, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
			if rcrs, err := c.crcrLister.List(labels.Everything()); err != nil {
				return err
			} else {
				obj.Summary = policyreportv1alpha2.PolicyReportSummary{}
				for _, rcr := range rcrs {
					obj.Summary = obj.Summary.Add(rcr.Summary)
				}
			}
			return nil
		},
	)
	return err
}

func (c *controller) rebuildReport(namespace string) error {
	_, err := controllerutils.CreateOrUpdate(
		namespace,
		c.polrLister.PolicyReports(namespace),
		c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace),
		func(obj *policyreportv1alpha2.PolicyReport) error {
			obj.SetNamespace(namespace)
			controllerutils.SetLabel(obj, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
			if rcrs, err := c.rcrLister.ReportChangeRequests(namespace).List(labels.Everything()); err != nil {
				return err
			} else {
				obj.Summary = policyreportv1alpha2.PolicyReportSummary{}
				for _, rcr := range rcrs {
					obj.Summary = obj.Summary.Add(rcr.Summary)
				}
			}
			return nil
		},
	)
	return err
}
