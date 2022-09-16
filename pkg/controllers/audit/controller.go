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
	add := func(obj interface{}) {
		selector := labels.Everything()
		requirement, err := policyLabelRequirementDoesNotExist(obj.(kyvernov1.PolicyInterface))
		if err != nil {
			logger.Error(err, "failed to create label selector")
		} else {
			selector = selector.Add(*requirement)
		}
		c.enqueue(selector)
	}
	update := func(_, obj interface{}) {
		selector := labels.Everything()
		requirement, err := policyLabelRequirementNotEquals(obj.(kyvernov1.PolicyInterface))
		if err != nil {
			logger.Error(err, "failed to create label selector")
		} else {
			selector = selector.Add(*requirement)
		}
		c.enqueue(selector)
	}
	delete := func(obj interface{}) {
		selector := labels.Everything()
		requirement, err := policyLabelRequirementExists(obj.(kyvernov1.PolicyInterface))
		if err != nil {
			logger.Error(err, "failed to create label selector")
		} else {
			selector = selector.Add(*requirement)
		}
		c.enqueue(selector)
	}
	controllerutils.AddEventHandlers(polInformer.Informer(), add, update, delete)
	controllerutils.AddEventHandlers(cpolInformer.Informer(), add, update, delete)
	controllerutils.AddDefaultEventHandlers(logger, rcrInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, crcrInformer.Informer(), c.queue)
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	go c.ticker(stopCh)
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) enqueue(selector labels.Selector) {
	rcrs, err := c.rcrLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to list rcrs")
	} else {
		for _, rcr := range rcrs {
			controllerutils.Enqueue(logger, c.queue, rcr, controllerutils.MetaNamespaceKey)
		}
	}
	crcrs, err := c.crcrLister.List(selector)
	if err != nil {
		logger.Error(err, "failed to list crcrs")
	} else {
		for _, crcr := range crcrs {
			controllerutils.Enqueue(logger, c.queue, crcr, controllerutils.MetaNamespaceKey)
		}
	}
}

func (c *controller) fetchPolicies(logger logr.Logger) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := c.polLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, pol)
		}
	}
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			policies = append(policies, cpol)
		}
	}
	return policies, nil
}

func removeNonBackgroundPolicies(logger logr.Logger, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var backgroundPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		if canBackgroundProcess(logger, pol) {
			backgroundPolicies = append(backgroundPolicies, pol)
		}
	}
	return backgroundPolicies
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

func (c *controller) reconcileReport(namespace, name string) error {
	// fetch report, if not found is not an error
	rcr, err := c.rcrLister.ReportChangeRequests(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	// load all policies
	policies, err := c.fetchPolicies(logger)
	if err != nil {
		return err
	}
	// load background policies
	backgroundPolicies := removeNonBackgroundPolicies(logger, policies...)
	// build label/policy maps
	labelPolicyMap := map[string]kyvernov1.PolicyInterface{}
	for _, policy := range policies {
		labelPolicyMap[policyLabel(policy)] = policy
	}
	labelBackgroundPolicyMap := map[string]kyvernov1.PolicyInterface{}
	for _, policy := range backgroundPolicies {
		labelBackgroundPolicyMap[policyLabel(policy)] = policy
	}
	// update report
	_, err = controllerutils.Update(c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(namespace), rcr,
		func(rcr *kyvernov1alpha2.ReportChangeRequest) error {
			rcr.SetNamespace(namespace)
			labels := controllerutils.SetLabel(rcr, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
			// check report policies versions against policies version
			toDelete := map[string]string{}
			var toCreate []kyvernov1.PolicyInterface
			for label := range labels {
				if isPolicyLabel(label) {
					// if the policy doesn't exist anymore
					if labelPolicyMap[label] == nil {
						if name, err := policyNameFromLabel(namespace, label); err != nil {
							return err
						} else {
							toDelete[name] = label
						}
					}
				}
			}
			for label, policy := range labelBackgroundPolicyMap {
				// if the background policy changed, we need to recreate entries
				if labels[label] != policy.GetResourceVersion() {
					if name, err := policyNameFromLabel(namespace, label); err != nil {
						return err
					} else {
						toDelete[name] = label
					}
					toCreate = append(toCreate, policy)
				}
			}
			// deletions
			for _, label := range toDelete {
				delete(labels, label)
			}
			var ruleResults []policyreportv1alpha2.PolicyReportResult
			for _, result := range rcr.Results {
				if _, ok := toDelete[result.Policy]; !ok {
					ruleResults = append(ruleResults, result)
				}
			}
			// creations
			if len(toCreate) > 0 {
				scanner := NewScanner(logger, c.client)
				owner := rcr.OwnerReferences[0]
				resource, err := c.client.GetResource(owner.APIVersion, owner.Kind, rcr.GetNamespace(), owner.Name)
				if err != nil {
					return err
				}
				for _, result := range scanner.Scan(*resource, toCreate...) {
					controllerutils.SetLabel(rcr, policyLabel(result.Policy), result.Policy.GetResourceVersion())
					ruleResults = append(ruleResults, toReportResults(result)...)
				}
			}
			// update results and summary
			rcr.Results = ruleResults
			rcr.Summary = CalculateSummary(ruleResults)
			return nil
		},
	)
	if err != nil {
		logger.Error(err, "failed to create or update rcr")
	}
	return nil
}

func (c *controller) reconcileClusterReport(name string) error {
	_, err := c.crcrLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	return nil
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.Info("reconciling ...")
	if namespace == "" {
		return c.reconcileClusterReport(name)
	} else {
		return c.reconcileReport(namespace, name)
	}
}

func (c *controller) sync() error {
	logger.Info("start sync ...")
	defer logger.Info("stop sync ...")
	policies, err := c.fetchPolicies(logger)
	if err != nil {
		return err
	}
	backgroundPolicies := removeNonBackgroundPolicies(logger, policies...)
	resources, err := c.fetchResources(logger, backgroundPolicies...)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.GetNamespace() == "" {
			name := "crcr-" + string(resource.GetUID())
			if _, err := c.crcrLister.Get(name); err != nil {
				if apierrors.IsNotFound(err) {
					_, err = controllerutils.CreateOrUpdate(
						name,
						c.crcrLister,
						c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests(),
						func(rcr *kyvernov1alpha2.ClusterReportChangeRequest) error {
							controllerutils.SetLabel(rcr, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
							controllerutils.SetOwner(rcr, resource.GetAPIVersion(), resource.GetKind(), resource.GetName(), resource.GetUID())
							return nil
						},
					)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else {
			name := "rcr-" + string(resource.GetUID())
			if _, err := c.rcrLister.ReportChangeRequests(resource.GetNamespace()).Get(name); err != nil {
				if apierrors.IsNotFound(err) {
					_, err = controllerutils.CreateOrUpdate(
						name,
						c.rcrLister.ReportChangeRequests(resource.GetNamespace()),
						c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(resource.GetNamespace()),
						func(rcr *kyvernov1alpha2.ReportChangeRequest) error {
							controllerutils.SetLabel(rcr, kyvernov1.ManagedByLabel, kyvernov1.KyvernoAppValue)
							controllerutils.SetOwner(rcr, resource.GetAPIVersion(), resource.GetKind(), resource.GetName(), resource.GetUID())
							return nil
						},
					)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
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
			err := c.sync()
			if err != nil {
				logger.Error(err, "sync failed")
			}
			c.enqueue(labels.Everything())
		case <-stopChan:
			return
		}
	}
}
