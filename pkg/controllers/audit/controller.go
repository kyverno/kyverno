package audit

import (
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1alpha2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
)

// TODO: skip resources to be filtered
// TODO: leader election
// TODO: admission scan refactor
// TODO: namespace hash

const (
	maxRetries = 10
	workers    = 10
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
	nsLister   corev1listers.NamespaceLister

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
	nsInformer corev1informers.NamespaceInformer,
) *controller {
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		polLister:     polInformer.Lister(),
		cpolLister:    cpolInformer.Lister(),
		rcrLister:     rcrInformer.Lister(),
		crcrLister:    crcrInformer.Lister(),
		nsLister:      nsInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
	}
	controllerutils.AddEventHandlers(polInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddEventHandlers(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddDefaultEventHandlers(logger, rcrInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, crcrInformer.Informer(), c.queue)
	// TODO we should also watch namespaces, if labels change
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	go c.background(stopCh)
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) addPolicy(obj interface{}) {
	selector := labels.Everything()
	requirement, err := policyLabelRequirementDoesNotExist(obj.(kyvernov1.PolicyInterface))
	if err != nil {
		logger.Error(err, "failed to create label selector")
	} else {
		selector = selector.Add(*requirement)
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
}

func (c *controller) updatePolicy(_, obj interface{}) {
	selector := labels.Everything()
	requirement, err := policyLabelRequirementNotEquals(obj.(kyvernov1.PolicyInterface))
	if err != nil {
		logger.Error(err, "failed to create label selector")
	} else {
		selector = selector.Add(*requirement)
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
}

func (c *controller) deletePolicy(obj interface{}) {
	selector := labels.Everything()
	requirement, err := policyLabelRequirementExists(obj.(kyvernov1.PolicyInterface))
	if err != nil {
		logger.Error(err, "failed to create label selector")
	} else {
		selector = selector.Add(*requirement)
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
}

func (c *controller) enqueue(selector labels.Selector) error {
	rcrs, err := c.rcrLister.List(selector)
	if err != nil {
		return err
	}
	for _, rcr := range rcrs {
		controllerutils.Enqueue(logger, c.queue, rcr, controllerutils.MetaNamespaceKey)
	}
	crcrs, err := c.crcrLister.List(selector)
	if err != nil {
		return err
	}
	for _, crcr := range crcrs {
		controllerutils.Enqueue(logger, c.queue, crcr, controllerutils.MetaNamespaceKey)
	}
	return nil
}

func (c *controller) fetchClusterPolicies(logger logr.Logger) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if cpols, err := c.cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			policies = append(policies, cpol)
		}
	}
	return policies, nil
}

func (c *controller) fetchPolicies(logger logr.Logger, namespace string) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := c.polLister.Policies(namespace).List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, pol)
		}
	}
	return policies, nil
}

func (c *controller) fetchResources(logger logr.Logger, policies ...kyvernov1.PolicyInterface) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured
	kinds := buildKindSet(logger, policies...)
	for kind := range kinds {
		list, err := c.client.ListResource("", kind, metav1.NamespaceAll, nil)
		if err != nil {
			logger.Error(err, "failed to list resources", "kind", kind)
			return nil, err
		}
		resources = append(resources, list.Items...)
	}
	return resources, nil
}

func (c *controller) reconcileReport(namespace, name string) error {
	return reconcileReport[kyvernov1alpha2.ReportChangeRequest](
		c,
		name,
		c.rcrLister.ReportChangeRequests(namespace),
		c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(namespace),
	)
}

func (c *controller) reconcileClusterReport(name string) error {
	return reconcileReport[kyvernov1alpha2.ClusterReportChangeRequest](
		c,
		name,
		c.crcrLister,
		c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests(),
	)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.V(2).Info("reconciling ...")
	if namespace == "" {
		return c.reconcileClusterReport(name)
	} else {
		return c.reconcileReport(namespace, name)
	}
}

func (c *controller) sync() error {
	logger.V(2).Info("start sync ...")
	defer logger.V(2).Info("stop sync ...")
	clusterPolicies, err := c.fetchClusterPolicies(logger)
	if err != nil {
		return err
	}
	policies, err := c.fetchPolicies(logger, metav1.NamespaceAll)
	if err != nil {
		return err
	}
	backgroundPolicies := removeNonBackgroundPolicies(logger, append(clusterPolicies, policies...)...)
	resources, err := c.fetchResources(logger, backgroundPolicies...)
	if err != nil {
		return err
	}
	var expectedRcrs []*kyvernov1alpha2.ReportChangeRequest
	var expectedCrcrs []*kyvernov1alpha2.ClusterReportChangeRequest
	for _, resource := range resources {
		if resource.GetNamespace() == "" {
			name := "crcr-" + string(resource.GetUID())
			if crcr, err := c.crcrLister.Get(name); err != nil {
				if apierrors.IsNotFound(err) {
					crcr, err := controllerutils.CreateOrUpdate(
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
					expectedCrcrs = append(expectedCrcrs, crcr)
				} else {
					return err
				}
			} else {
				expectedCrcrs = append(expectedCrcrs, crcr)
			}
		} else {
			name := "rcr-" + string(resource.GetUID())
			if rcr, err := c.rcrLister.ReportChangeRequests(resource.GetNamespace()).Get(name); err != nil {
				if apierrors.IsNotFound(err) {
					rcr, err := controllerutils.CreateOrUpdate(
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
					expectedRcrs = append(expectedRcrs, rcr)
				} else {
					return err
				}
			} else {
				expectedRcrs = append(expectedRcrs, rcr)
			}
		}
	}
	actualRcrs, err := c.rcrLister.List(labels.Everything())
	if err != nil {
		return err
	}
	actualCrcrs, err := c.crcrLister.List(labels.Everything())
	if err != nil {
		return err
	}
	if err := controllerutils.Cleanup(actualCrcrs, expectedCrcrs, c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests()); err != nil {
		return err
	}
	namespaces := sets.NewString()
	for _, rcr := range actualRcrs {
		namespaces.Insert(rcr.GetNamespace())
	}
	for _, rcr := range expectedRcrs {
		namespaces.Insert(rcr.GetNamespace())
	}
	for _, namespace := range namespaces.List() {
		var actual []*kyvernov1alpha2.ReportChangeRequest
		for _, rcr := range actualRcrs {
			if rcr.GetNamespace() == namespace {
				actual = append(actual, rcr)
			}
		}
		var expected []*kyvernov1alpha2.ReportChangeRequest
		for _, rcr := range expectedRcrs {
			if rcr.GetNamespace() == namespace {
				expected = append(expected, rcr)
			}
		}
		if err := controllerutils.Cleanup(actual, expected, c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(namespace)); err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) background(stopChan <-chan struct{}) {
	sync := time.NewTicker(1 * time.Minute)
	requeue := time.NewTicker(1 * time.Minute)
	defer sync.Stop()
	defer requeue.Stop()
	for {
		select {
		case <-sync.C:
			err := c.sync()
			if err != nil {
				logger.Error(err, "sync failed")
			}
		case <-requeue.C:
			c.enqueue(labels.Everything())
		case <-stopChan:
			return
		}
	}
}
