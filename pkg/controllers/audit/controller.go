package audit

import (
	"context"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"
)

// TODO: managed by kyverno label
// TODO: deep copy if coming from cache

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// clients
	client versioned.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	rcrLister  kyvernov1alpha2listers.ReportChangeRequestLister
	crcrLister kyvernov1alpha2listers.ClusterReportChangeRequestLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client versioned.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
) *controller {
	c := controller{
		client:     client,
		polLister:  polInformer.Lister(),
		cpolLister: cpolInformer.Lister(),
		rcrLister:  rcrInformer.Lister(),
		crcrLister: crcrInformer.Lister(),
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")
	cpol, err := c.cpolLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// cpol deleted
			return nil
		} else {
			return err
		}
	}
	// TODO: use labels for matching
	rcrs, err := c.rcrLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, rcr := range rcrs {
		matched := true
		labels := rcr.GetLabels()
		if labels != nil {
			if labels[key] == cpol.GetResourceVersion() {
				matched = false
			}
		}
		if matched {
			logger.Info("needs processing ...", "rcr", rcr)
		}
		rcr = rcr.DeepCopy()
		controllerutils.SetLabel(rcr, "kyverno.io/"+key, cpol.GetResourceVersion())
		_, err = c.client.KyvernoV1alpha2().ReportChangeRequests(rcr.Namespace).Update(context.TODO(), rcr, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "failed to update rcr")
		}
	}
	return nil
}
