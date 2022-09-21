package audit

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"sync"
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
	"github.com/kyverno/kyverno/pkg/engine/response"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/metadata"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
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
	client         dclient.Interface
	metadataClient metadata.Interface
	kyvernoClient  versioned.Interface

	// listers
	polLister  kyvernov1listers.PolicyLister
	cpolLister kyvernov1listers.ClusterPolicyLister
	rcrLister  kyvernov1alpha2listers.ReportChangeRequestLister
	crcrLister kyvernov1alpha2listers.ClusterReportChangeRequestLister
	nsLister   corev1listers.NamespaceLister

	// queue
	queue workqueue.RateLimitingInterface

	lock              sync.Mutex
	metadataInformers map[schema.GroupVersionResource]*informer
}

type informer struct {
	informer informers.GenericInformer
	gvk      schema.GroupVersionKind
	stop     <-chan struct{}
}

func NewController(
	client dclient.Interface,
	metadataClient metadata.Interface,
	kyvernoClient versioned.Interface,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	rcrInformer kyvernov1alpha2informers.ReportChangeRequestInformer,
	crcrInformer kyvernov1alpha2informers.ClusterReportChangeRequestInformer,
	nsInformer corev1informers.NamespaceInformer,
) *controller {
	c := controller{
		client:            client,
		metadataClient:    metadataClient,
		kyvernoClient:     kyvernoClient,
		polLister:         polInformer.Lister(),
		cpolLister:        cpolInformer.Lister(),
		rcrLister:         rcrInformer.Lister(),
		crcrLister:        crcrInformer.Lister(),
		nsLister:          nsInformer.Lister(),
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName),
		metadataInformers: map[schema.GroupVersionResource]*informer{},
	}
	controllerutils.AddEventHandlers(polInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddEventHandlers(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy)
	controllerutils.AddDefaultEventHandlers(logger, rcrInformer.Informer(), c.queue)
	controllerutils.AddDefaultEventHandlers(logger, crcrInformer.Informer(), c.queue)
	// TODO we should also watch namespaces, if labels change
	return &c
}

func (c *controller) Run(stopCh <-chan struct{}) {
	// go c.background(stopCh)
	controllerutils.Run(controllerName, logger, c.queue, workers, maxRetries, c.reconcile, stopCh /*, c.configmapSynced*/)
}

func (c *controller) updateMetadataInformers() {
	c.lock.Lock()
	defer c.lock.Unlock()

	clusterPolicies, err := c.fetchClusterPolicies(logger)
	if err != nil {
		logger.Error(err, "failed to list cluster policies")
	}
	policies, err := c.fetchPolicies(logger, metav1.NamespaceAll)
	if err != nil {
		logger.Error(err, "failed to list policies")
	}
	kinds := buildKindSet(logger, removeNonBackgroundPolicies(logger, append(clusterPolicies, policies...)...)...)
	gvrs := map[string]schema.GroupVersionResource{}
	for _, kind := range kinds.List() {
		gvr, err := c.client.Discovery().GetGVRFromKind(kind)
		if err == nil {
			gvrs[kind] = gvr
		} else {
			logger.Error(err, "failed to get gvr from kind", "kind", kind)
		}
	}
	for kind, gvr := range gvrs {
		i := c.metadataInformers[gvr]
		if i == nil {
			logger.Info("start metadata informer ...", "gvr", gvr)
			i = &informer{
				gvk: gvr.GroupVersion().WithKind(kind),
				informer: metadatainformers.NewFilteredMetadataInformer(
					c.metadataClient,
					gvr,
					"",
					time.Minute*10,
					nil,
					nil,
				),
				stop: make(chan struct{}),
			}
			i.informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc:    c.addResource,
				UpdateFunc: c.updateResource,
				DeleteFunc: c.deleteResource,
			})
			go i.informer.Informer().Run(i.stop)
			c.metadataInformers[gvr] = i
		}
	}
	// TODO: cleanup
}

func (c *controller) addResource(obj interface{}) {
	selector := labels.Everything()
	resource := obj.(metav1.Object)
	requirement, err := resourceLabelRequirementUidEquals(resource)
	if err != nil {
		logger.Error(err, "failed to create label selector")
	} else {
		selector = selector.Add(*requirement)
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
	if resource.GetNamespace() == "" {
		c.queue.Add(string(resource.GetUID()))
	} else {
		c.queue.Add(resource.GetNamespace() + "/" + string(resource.GetUID()))
	}
}

func (c *controller) updateResource(_, obj interface{}) {
	selector := labels.Everything()
	resource := obj.(metav1.Object)
	requirement, err := resourceLabelRequirementUidEquals(obj.(metav1.Object))
	if err != nil {
		logger.Error(err, "failed to create label selector")
	} else {
		selector = selector.Add(*requirement)
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
	if resource.GetNamespace() == "" {
		c.queue.Add(string(resource.GetUID()))
	} else {
		c.queue.Add(resource.GetNamespace() + "/" + string(resource.GetUID()))
	}
}

func (c *controller) deleteResource(obj interface{}) {
	selector := labels.Everything()
	resource := obj.(metav1.Object)
	requirement, err := resourceLabelRequirementUidEquals(obj.(metav1.Object))
	if err != nil {
		logger.Error(err, "failed to create label selector")
	} else {
		selector = selector.Add(*requirement)
	}
	if err := c.enqueue(selector); err != nil {
		logger.Error(err, "failed to enqueue")
	}
	if resource.GetNamespace() == "" {
		c.queue.Add(string(resource.GetUID()))
	} else {
		c.queue.Add(resource.GetNamespace() + "/" + string(resource.GetUID()))
	}
}

func (c *controller) addPolicy(obj interface{}) {
	c.updateMetadataInformers()
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
	c.updateMetadataInformers()
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
	c.updateMetadataInformers()
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
	logger.Info("enqueuing ...", "selector", selector.String())
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
	return ReconcileReport[kyvernov1alpha2.ReportChangeRequest](
		c,
		name,
		c.rcrLister.ReportChangeRequests(namespace),
		c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(namespace),
	)
}

func (c *controller) reconcileClusterReport(name string) error {
	return ReconcileReport[kyvernov1alpha2.ClusterReportChangeRequest](
		c,
		name,
		c.crcrLister,
		c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests(),
	)
}

func (c *controller) getReport(namespace, name string) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	if namespace == "" {
		if report, err := c.crcrLister.Get(name); err != nil {
			return nil, err
		} else {
			return report.DeepCopy(), nil
		}
	} else {
		if report, err := c.rcrLister.ReportChangeRequests(namespace).Get(name); err != nil {
			return nil, err
		} else {
			return report.DeepCopy(), nil
		}
	}
}

func (c *controller) getResource(uid types.UID) (metav1.Object, schema.GroupVersionKind, error) {
	for _, i := range c.metadataInformers {
		list, err := i.informer.Lister().List(labels.Everything())
		if err != nil {
			return nil, schema.GroupVersionKind{}, err
		}
		for _, o := range list {
			if o.(metav1.Object).GetUID() == uid {
				return o.(metav1.Object), i.gvk, nil
			}
		}
	}
	return nil, schema.GroupVersionKind{}, nil
}

func (c *controller) createReport(namespace, name string) error {
	resource, gvk, err := c.getResource(types.UID(name))
	if err == nil && resource != nil {
		if namespace == "" {
			report := &kyvernov1alpha2.ClusterReportChangeRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}
			BuildReport(report, gvk.Group, gvk.Version, gvk.Kind, resource)
			controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, resource.GetName(), resource.GetUID())
			_, err = c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests().Create(context.TODO(), report, metav1.CreateOptions{})
		} else {
			report := &kyvernov1alpha2.ReportChangeRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
			BuildReport(report, gvk.Group, gvk.Version, gvk.Kind, resource)
			controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, resource.GetName(), resource.GetUID())
			_, err = c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(namespace).Create(context.TODO(), report, metav1.CreateOptions{})
		}
	}
	return err
}

func (c *controller) setOwner(report kyvernov1alpha2.ReportChangeRequestInterface) error {
	resource, gvk, err := c.getResource(types.UID(report.GetLabels()["audit.kyverno.io/resource.uid"]))
	if err == nil && resource != nil {
		controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, resource.GetName(), resource.GetUID())
	}
	return c.saveReport(report)
}

func (c *controller) saveReport(report kyvernov1alpha2.ReportChangeRequestInterface) error {
	switch v := report.(type) {
	case *kyvernov1alpha2.ReportChangeRequest:
		_, err := c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(report.GetNamespace()).Update(context.TODO(), v, metav1.UpdateOptions{})
		return err
	case *kyvernov1alpha2.ClusterReportChangeRequest:
		_, err := c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests().Update(context.TODO(), v, metav1.UpdateOptions{})
		return err
	default:
		return errors.New("unknow type")
	}
}

func (c *controller) deepCopy(report kyvernov1alpha2.ReportChangeRequestInterface) kyvernov1alpha2.ReportChangeRequestInterface {
	switch v := report.(type) {
	case *kyvernov1alpha2.ReportChangeRequest:
		return v.DeepCopy()
	case *kyvernov1alpha2.ClusterReportChangeRequest:
		return v.DeepCopy()
	default:
		return nil
	}
}

func (c *controller) computeReport(before kyvernov1alpha2.ReportChangeRequestInterface) error {
	report := c.deepCopy(before)
	namespace := report.GetNamespace()
	labels := report.GetLabels()
	// load all policies
	policies, err := c.fetchClusterPolicies(logger)
	if err != nil {
		return err
	}
	if namespace != "" {
		pols, err := c.fetchPolicies(logger, namespace)
		if err != nil {
			return err
		}
		policies = append(policies, pols...)
	}
	// 	load background policies
	backgroundPolicies := removeNonBackgroundPolicies(logger, policies...)
	resource, gvk, err := c.getResource(types.UID(labels["audit.kyverno.io/resource.uid"]))
	if err != nil {
		return err
	}
	if resource == nil {
		return nil
	}
	//	if the resource changed, we need to rebuild the report
	if resource.GetResourceVersion() != labels["audit.kyverno.io/resource.version"] {
		scanner := NewScanner(logger, c.client)
		resource, err := c.client.GetResource(gvk.GroupVersion().String(), gvk.Kind, report.GetNamespace(), resource.GetName())
		if err != nil {
			return err
		}
		var nsLabels map[string]string
		if namespace != "" {
			ns, err := c.nsLister.Get(namespace)
			if err != nil {
				return err
			}
			nsLabels = ns.GetLabels()
		}
		var responses []*response.EngineResponse
		for _, result := range scanner.ScanResource(*resource, nsLabels, backgroundPolicies...) {
			if result.Error != nil {
				logger.Error(result.Error, "failed to apply policy")
				// return nil, result.Error
			} else {
				responses = append(responses, result.EngineResponse)
			}
		}
		BuildReport(report, gvk.Group, gvk.Version, gvk.Kind, resource, responses...)
	} else {
		expected := map[string]kyvernov1.PolicyInterface{}
		for _, policy := range backgroundPolicies {
			expected[policyLabel(policy)] = policy
		}
		toDelete := map[string]string{}
		for label := range labels {
			if isPolicyLabel(label) {
				// if the policy doesn't exist anymore
				if expected[label] == nil {
					if name, err := policyNameFromLabel(namespace, label); err != nil {
						return err
					} else {
						toDelete[name] = label
					}
				}
			}
		}
		var toCreate []kyvernov1.PolicyInterface
		for label, policy := range expected {
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
		var ruleResults []policyreportv1alpha2.PolicyReportResult
		// deletions
		for _, label := range toDelete {
			delete(labels, label)
		}
		for _, result := range report.GetResults() {
			if _, ok := toDelete[result.Policy]; !ok {
				ruleResults = append(ruleResults, result)
			}
		}
		// creations
		if len(toCreate) > 0 {
			scanner := NewScanner(logger, c.client)
			resource, err := c.client.GetResource(gvk.GroupVersion().String(), gvk.Kind, report.GetNamespace(), resource.GetName())
			if err != nil {
				return err
			}
			controllerutils.SetLabel(report, "audit.kyverno.io/resource.gvk.group", gvk.Group)
			controllerutils.SetLabel(report, "audit.kyverno.io/resource.gvk.version", gvk.Version)
			controllerutils.SetLabel(report, "audit.kyverno.io/resource.gvk.kind", gvk.Kind)
			controllerutils.SetLabel(report, "audit.kyverno.io/resource.version", resource.GetResourceVersion())
			controllerutils.SetLabel(report, "audit.kyverno.io/resource.generation", strconv.FormatInt(resource.GetGeneration(), 10))
			var nsLabels map[string]string
			if namespace != "" {
				ns, err := c.nsLister.Get(namespace)
				if err != nil {
					return err
				}
				nsLabels = ns.GetLabels()
			}
			for _, result := range scanner.ScanResource(*resource, nsLabels, toCreate...) {
				if result.Error != nil {
					return result.Error
				} else {
					controllerutils.SetLabel(report, policyLabel(result.EngineResponse.Policy), result.EngineResponse.Policy.GetResourceVersion())
					ruleResults = append(ruleResults, toReportResults(result)...)
				}
			}
		}
		// update results and summary
		SortReportResults(ruleResults)
		report.SetResults(ruleResults)
		report.SetSummary(CalculateSummary(ruleResults))
	}
	if reflect.DeepEqual(before, report) {
		return nil
	}
	return c.saveReport(report)
}

func (c *controller) reconcile(key, namespace, name string) error {
	logger := logger.WithValues("key", key, "namespace", namespace, "name", name)
	logger.V(2).Info("reconciling ...")
	report, err := c.getReport(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.createReport(namespace, name)
		}
		return err
	}
	if len(report.GetOwnerReferences()) == 0 {
		return c.setOwner(report)
	}
	//	if the report is coming from an admission request, we don't want to mutate it
	if report.GetLabels()["audit.kyverno.io/request.uid"] != "" {
		return nil
	}
	return c.computeReport(report)
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
			name := string(resource.GetUID())
			if crcr, err := c.crcrLister.Get(name); err != nil {
				if apierrors.IsNotFound(err) {
					crcr, err := controllerutils.CreateOrUpdate(
						name,
						c.crcrLister,
						c.kyvernoClient.KyvernoV1alpha2().ClusterReportChangeRequests(),
						func(rcr *kyvernov1alpha2.ClusterReportChangeRequest) error {
							controllerutils.SetLabel(rcr, kyvernov1.LabelAppManagedBy, kyvernov1.ValueKyvernoApp)
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.uid", string(resource.GetUID()))
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.namespace", resource.GetNamespace())
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.version", resource.GetResourceVersion())
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.generation", strconv.FormatInt(resource.GetGeneration(), 10))
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
			name := string(resource.GetUID())
			if rcr, err := c.rcrLister.ReportChangeRequests(resource.GetNamespace()).Get(name); err != nil {
				if apierrors.IsNotFound(err) {
					rcr, err := controllerutils.CreateOrUpdate(
						name,
						c.rcrLister.ReportChangeRequests(resource.GetNamespace()),
						c.kyvernoClient.KyvernoV1alpha2().ReportChangeRequests(resource.GetNamespace()),
						func(rcr *kyvernov1alpha2.ReportChangeRequest) error {
							controllerutils.SetLabel(rcr, kyvernov1.LabelAppManagedBy, kyvernov1.ValueKyvernoApp)
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.uid", string(resource.GetUID()))
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.namespace", resource.GetNamespace())
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.version", resource.GetResourceVersion())
							controllerutils.SetLabel(rcr, "audit.kyverno.io/resource.generation", strconv.FormatInt(resource.GetGeneration(), 10))
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
