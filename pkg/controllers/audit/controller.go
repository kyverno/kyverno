package audit

import (
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/workqueue"
)

// TODO: skip resources to be filtered
// TODO: get discovery schema
// TODO: clean up dangling reports

// DONE: cache background policies
// DONE: validate variables
// DONE: transmit logger
// DONE: build kinds
// DONE: filter out unnecessary rules
// DONE: managed by kyverno label
// DONE: deep copy if coming from cache
// DONE: compare in createorupdate before calling
// DONE: convert scan results
// DONE: sort results
// DONE: policiy removal

const (
	maxRetries = 10
	workers    = 3
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	rcrLister  kyvernov1alpha2listers.ReportChangeRequestLister
	crcrLister kyvernov1alpha2listers.ClusterReportChangeRequestLister

	// queue
	queue workqueue.RateLimitingInterface
}

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
) *controller {
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		polLister:     polInformer.Lister(),
		cpolLister:    cpolInformer.Lister(),
		rcrLister:     rcrInformer.Lister(),
		crcrLister:    crcrInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddDefaultEventHandlers(logger, polInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, cpolInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	go c.ticker(stopCh)
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) fetchBackgroundPolicies(logger logr.Logger) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := c.polLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			if canBackgroundProcess(logger, pol) {
				policies = append(policies, pol.DeepCopy())
			}
		}
	}
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			if canBackgroundProcess(logger, cpol) {
				policies = append(policies, cpol.DeepCopy())
			}
		}
	}
	return policies, nil
}

func (c *controller) fetchResources(logger logr.Logger, policies ...kyvernov1.PolicyInterface) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured
	kinds := buildKindSet(logger, policies...)
	for kind := range kinds {
		list, err := c.client.ListResource("", kind, "" /*labelSelector*/, nil)
		if err != nil {
			logger.Error(err, "failed to list resources", "kind", kind)
			return nil, err
		}
		resources = append(resources, list.Items...)
	}
	return resources, nil
}

func (c *controller) runScan(logger logr.Logger) error {
	logger.Info("start scan ...")
	defer logger.Info("stop scan ...")
	policies, err := c.fetchBackgroundPolicies(logger)
	if err != nil {
		return err
	}
	resources, err := c.fetchResources(logger, policies...)
	if err != nil {
		return err
	}
	// run validation for all resources against all policies
	scanner := NewScanner(logger, c.client)
	for _, resource := range resources {
		namespace := resource.GetNamespace()
		var name string
		if namespace == "" {
			name = "crcr-" + string(resource.GetUID())
		} else {
			name = "rcr-" + string(resource.GetUID())
		}
		scanResult := scanner.Scan(resource, policies...)
		_, err := controllerutils.CreateOrUpdate(
			name,
			c.rcrLister.ReportChangeRequests(resource.GetNamespace()),
			c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(resource.GetNamespace()),
			func(obj *kyvernov1alpha2.ReportChangeRequest) error {
				obj.SetNamespace(resource.GetNamespace())
				controllerutils.SetLabel(obj, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
				controllerutils.SetOwner(obj, resource.GetAPIVersion(), resource.GetKind(), resource.GetName(), resource.GetUID())
				var ruleResults []policyreportv1alpha2.PolicyReportResult
				for _, result := range scanResult {
					controllerutils.SetLabel(obj, policyLabelPrefix(result.Policy)+"/"+result.Policy.GetName(), result.Policy.GetResourceVersion())
					ruleResults = append(ruleResults, toReportResults(result)...)
				}
				SortReportResults(ruleResults)
				obj.Results = ruleResults
				obj.Summary = CalculateSummary(ruleResults)
				return nil
			},
		)
		if err != nil {
			logger.Error(err, "failed to create or update rcr")
		}
	}
	return nil
}

func (c *controller) getPolicy(namespace, name string) (kyvernov1.PolicyInterface, error) {
	if namespace == "" {
		return c.cpolLister.Get(name)
	} else {
		return c.polLister.Policies(namespace).Get(name)
	}
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")
	_, err := c.getPolicy(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			rcrs, err := c.rcrLister.List(labels.Everything())
			if err != nil {
				return nil
			}
			for _, rcr := range rcrs {
				_, err = controllerutils.Update(
					c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(rcr.GetNamespace()),
					rcr,
					func(rcr *kyvernov1alpha2.ReportChangeRequest) error {
						var ruleResults []policyreportv1alpha2.PolicyReportResult
						for _, result := range rcr.Results {
							if result.Policy != key {
								ruleResults = append(ruleResults, result)
							}
						}
						rcr.Results = ruleResults
						rcr.Summary = CalculateSummary(ruleResults)
						return nil
					},
				)
				if err != nil {
					return err
				}
			}
		} else {
			return err
		}
	}
	return nil
}

func (c *controller) ticker(stopChan <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.runScan(logger)
		case <-stopChan:
			return
		}
	}
}
