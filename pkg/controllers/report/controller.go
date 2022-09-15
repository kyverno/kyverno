package report

import (
	"context"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	policyreportv1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
// TODO: controllerutils.CreateOrUpdate

// DONE: cpol aggregation
// DONE: managed by kyverno label
// DONE: deep copy if coming from cache

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
	report, err := c.cpolrLister.Get(cpolrName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			report = &policyreportv1alpha2.ClusterPolicyReport{}
			report.SetName(cpolrName)
		} else {
			return err
		}
	} else {
		report = report.DeepCopy()
	}
	controllerutils.SetLabel(report, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
	if rcrs, err := c.crcrLister.List(labels.Everything()); err != nil {
		return err
	} else {
		report.Summary = policyreportv1alpha2.PolicyReportSummary{}
		for _, rcr := range rcrs {
			report.Summary = report.Summary.Add(rcr.Summary)
		}
	}
	if report.GetResourceVersion() == "" {
		_, err = c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Create(context.TODO(), report, metav1.CreateOptions{})
	} else {
		_, err = c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), report, metav1.UpdateOptions{})
	}
	return err
}

func (c *controller) rebuildReport(namespace string) error {
	report, err := c.polrLister.PolicyReports(namespace).Get(namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			report = &policyreportv1alpha2.PolicyReport{}
			report.SetName(namespace)
			report.SetNamespace(namespace)
		} else {
			return err
		}
	} else {
		report = report.DeepCopy()
	}
	controllerutils.SetLabel(report, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
	if rcrs, err := c.rcrLister.ReportChangeRequests(namespace).List(labels.Everything()); err != nil {
		return err
	} else {
		report.Summary = policyreportv1alpha2.PolicyReportSummary{}
		for _, rcr := range rcrs {
			report.Summary = report.Summary.Add(rcr.Summary)
		}
	}
	if report.GetResourceVersion() == "" {
		_, err = c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Create(context.TODO(), report, metav1.CreateOptions{})
	} else {
		_, err = c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Update(context.TODO(), report, metav1.UpdateOptions{})
	}
	return err
}
