package report

import (
	"context"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
	rcrInformer  kyvernov1alpha2informers.ReportChangeRequestInformer
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(client versioned.Interface, rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer, crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer) *controller {
	c := controller{
		client:       client,
		rcrInformer:  rcrInformer,
		crcrInformer: crcrInformer,
		queue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, rcrInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, crcrInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")

	return c.rebuildReport(namespace)
}

func (c *controller) rebuildReport(namespace string) error {
	// TODO: use a lister
	report, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			report = &policyreportv1alpha2.PolicyReport{}
			report.SetName(namespace)
			report.SetNamespace(namespace)
		} else {
			return err
		}
	}
	if rcrs, err := c.rcrInformer.Lister().ReportChangeRequests(namespace).List(labels.Everything()); err != nil {
		return err
	} else {
		report.Summary = policyreportv1alpha2.PolicyReportSummary{}
		for _, rcr := range rcrs {
			report.Summary = report.Summary.Add(rcr.Summary)
		}
		// TODO: aggregate results
	}
	if report.GetResourceVersion() == "" {
		_, err = c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Create(context.TODO(), report, metav1.CreateOptions{})
	} else {
		_, err = c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Update(context.TODO(), report, metav1.UpdateOptions{})
	}
	return err
}
