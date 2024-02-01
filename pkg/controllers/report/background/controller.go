package background

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers                = 2
	ControllerName         = "background-scan-controller"
	maxRetries             = 10
	annotationLastScanTime = "audit.kyverno.io/last-scan-time"
	enqueueDelay           = 30 * time.Second
)

type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	engine        engineapi.Engine

	// listers
	polLister        kyvernov1listers.PolicyLister
	cpolLister       kyvernov1listers.ClusterPolicyLister
	polexLister      kyvernov2beta1listers.PolicyExceptionLister
	vapLister        admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyLister
	vapBindingLister admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyBindingLister
	bgscanrLister    cache.GenericLister
	cbgscanrLister   cache.GenericLister
	nsLister         corev1listers.NamespaceLister

	// queue
	queue workqueue.RateLimitingInterface

	// cache
	metadataCache resource.MetadataCache
	forceDelay    time.Duration

	// config
	config        config.Configuration
	jp            jmespath.Interface
	eventGen      event.Interface
	policyReports bool
}

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	engine engineapi.Engine,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polexInformer kyvernov2beta1informers.PolicyExceptionInformer,
	vapInformer admissionregistrationv1alpha1informers.ValidatingAdmissionPolicyInformer,
	vapBindingInformer admissionregistrationv1alpha1informers.ValidatingAdmissionPolicyBindingInformer,
	nsInformer corev1informers.NamespaceInformer,
	metadataCache resource.MetadataCache,
	forceDelay time.Duration,
	config config.Configuration,
	jp jmespath.Interface,
	eventGen event.Interface,
	policyReports bool,
) controllers.Controller {
	ephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName)
	c := controller{
		client:         client,
		kyvernoClient:  kyvernoClient,
		engine:         engine,
		polLister:      polInformer.Lister(),
		cpolLister:     cpolInformer.Lister(),
		polexLister:    polexInformer.Lister(),
		bgscanrLister:  ephrInformer.Lister(),
		cbgscanrLister: cephrInformer.Lister(),
		nsLister:       nsInformer.Lister(),
		queue:          queue,
		metadataCache:  metadataCache,
		forceDelay:     forceDelay,
		config:         config,
		jp:             jp,
		eventGen:       eventGen,
		policyReports:  policyReports,
	}
	if vapInformer != nil {
		c.vapLister = vapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(vapInformer.Informer(), c.addVAP, c.updateVAP, c.deleteVAP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if vapBindingInformer != nil {
		c.vapBindingLister = vapBindingInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(vapBindingInformer.Informer(), c.addVAPBinding, c.updateVAPBinding, c.deleteVAPBinding); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if _, err := controllerutils.AddEventHandlersT(polInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(cpolInformer.Informer(), c.addPolicy, c.updatePolicy, c.deletePolicy); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(polexInformer.Informer(), c.addException, c.updateException, c.deleteException); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	c.metadataCache.AddEventHandler(func(eventType resource.EventType, uid types.UID, _ schema.GroupVersionKind, res resource.Resource) {
		// if it's a deletion, nothing to do
		if eventType == resource.Deleted {
			return
		}
		if res.Namespace == "" {
			c.queue.AddAfter(string(uid), enqueueDelay)
		} else {
			c.queue.AddAfter(res.Namespace+"/"+string(uid), enqueueDelay)
		}
	})
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	logger.Info("background scan", "interval", c.forceDelay.Abs().String())
	controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue, workers, maxRetries, c.reconcile)
}

func (c *controller) addPolicy(obj kyvernov1.PolicyInterface) {
	c.enqueueResources()
}

func (c *controller) updatePolicy(old, obj kyvernov1.PolicyInterface) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deletePolicy(obj kyvernov1.PolicyInterface) {
	c.enqueueResources()
}

func (c *controller) addException(obj *kyvernov2beta1.PolicyException) {
	c.enqueueResources()
}

func (c *controller) updateException(old, obj *kyvernov2beta1.PolicyException) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteException(obj *kyvernov2beta1.PolicyException) {
	c.enqueueResources()
}

func (c *controller) addVAP(obj *admissionregistrationv1alpha1.ValidatingAdmissionPolicy) {
	c.enqueueResources()
}

func (c *controller) updateVAP(old, obj *admissionregistrationv1alpha1.ValidatingAdmissionPolicy) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteVAP(obj *admissionregistrationv1alpha1.ValidatingAdmissionPolicy) {
	c.enqueueResources()
}

func (c *controller) addVAPBinding(obj *admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding) {
	c.enqueueResources()
}

func (c *controller) updateVAPBinding(old, obj *admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteVAPBinding(obj *admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding) {
	c.enqueueResources()
}

func (c *controller) enqueueResources() {
	for _, key := range c.metadataCache.GetAllResourceKeys() {
		c.queue.Add(key)
	}
}

func (c *controller) getReport(ctx context.Context, namespace, name string) (kyvernov1alpha2.ReportInterface, error) {
	if namespace == "" {
		return c.kyvernoClient.ReportsV1().ClusterEphemeralReports().Get(ctx, name, metav1.GetOptions{})
	} else {
		return c.kyvernoClient.ReportsV1().EphemeralReports(namespace).Get(ctx, name, metav1.GetOptions{})
	}
}

func (c *controller) getMeta(namespace, name string) (metav1.Object, error) {
	if namespace == "" {
		obj, err := c.cbgscanrLister.Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(metav1.Object), err
	} else {
		obj, err := c.bgscanrLister.ByNamespace(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(metav1.Object), err
	}
}

func (c *controller) needsReconcile(namespace, name, hash string, exceptions []kyvernov2beta1.PolicyException, bindings []admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding, policies ...engineapi.GenericPolicy) (bool, bool, error) {
	// if the reportMetadata does not exist, we need a full reconcile
	reportMetadata, err := c.getMeta(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, true, nil
		}
		return false, false, err
	}
	// if the resource changed, we need a full reconcile
	if !reportutils.CompareHash(reportMetadata, hash) {
		return true, true, nil
	}
	// if the last scan time is older than recomputation interval, we need a full reconcile
	reportAnnotations := reportMetadata.GetAnnotations()
	if reportAnnotations == nil || reportAnnotations[annotationLastScanTime] == "" {
		return true, true, nil
	} else {
		annTime, err := time.Parse(time.RFC3339, reportAnnotations[annotationLastScanTime])
		if err != nil {
			logger.Error(err, "failed to parse last scan time annotation", "namespace", namespace, "name", name, "hash", hash)
			return true, true, nil
		}
		if time.Now().After(annTime.Add(c.forceDelay)) {
			return true, true, nil
		}
	}
	// if a policy or an exception changed, we need a partial reconcile
	expected := map[string]string{}
	for _, policy := range policies {
		expected[reportutils.PolicyLabel(policy)] = policy.GetResourceVersion()
	}
	for _, exception := range exceptions {
		expected[reportutils.PolicyExceptionLabel(exception)] = exception.GetResourceVersion()
	}
	for _, binding := range bindings {
		expected[reportutils.ValidatingAdmissionPolicyBindingLabel(binding)] = binding.GetResourceVersion()
	}
	actual := map[string]string{}
	for key, value := range reportMetadata.GetLabels() {
		if reportutils.IsPolicyLabel(key) {
			actual[key] = value
		}
	}
	if !datautils.DeepEqual(expected, actual) {
		return true, false, nil
	}
	// no need to reconcile
	return false, false, nil
}

func (c *controller) reconcileReport(
	ctx context.Context,
	namespace string,
	name string,
	full bool,
	uid types.UID,
	gvk schema.GroupVersionKind,
	resource resource.Resource,
	exceptions []kyvernov2beta1.PolicyException,
	bindings []admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding,
	policies ...engineapi.GenericPolicy,
) error {
	// namespace labels to be used by the scanner
	var nsLabels map[string]string
	if namespace != "" {
		ns, err := c.nsLister.Get(namespace)
		if err != nil {
			return err
		}
		nsLabels = ns.GetLabels()
	}
	// load target resource
	target, err := c.client.GetResource(ctx, gvk.GroupVersion().String(), gvk.Kind, resource.Namespace, resource.Name)
	if err != nil {
		return err
	}
	// load observed report
	observed, err := c.getReport(ctx, namespace, name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		observed = reportutils.NewBackgroundScanReport(namespace, name, gvk, resource.Name, uid)
	}
	// build desired report
	expected := map[string]string{}
	for _, policy := range policies {
		expected[reportutils.PolicyLabel(policy)] = policy.GetResourceVersion()
	}
	for _, exception := range exceptions {
		expected[reportutils.PolicyExceptionLabel(exception)] = exception.GetResourceVersion()
	}
	for _, binding := range bindings {
		expected[reportutils.ValidatingAdmissionPolicyBindingLabel(binding)] = binding.GetResourceVersion()
	}
	actual := map[string]string{}
	for key, value := range observed.GetLabels() {
		if reportutils.IsPolicyLabel(key) {
			actual[key] = value
		}
	}
	var ruleResults []policyreportv1alpha2.PolicyReportResult
	if !full {
		policyNameToLabel := map[string]string{}
		for _, policy := range policies {
			var key string
			var err error
			if policy.GetType() == engineapi.KyvernoPolicyType {
				key, err = cache.MetaNamespaceKeyFunc(policy.AsKyvernoPolicy())
			} else {
				key, err = cache.MetaNamespaceKeyFunc(policy.AsValidatingAdmissionPolicy())
			}
			if err != nil {
				return err
			}
			policyNameToLabel[key] = reportutils.PolicyLabel(policy)
		}
		for _, exception := range exceptions {
			key, err := cache.MetaNamespaceKeyFunc(exception)
			if err != nil {
				return err
			}
			policyNameToLabel[key] = reportutils.PolicyExceptionLabel(exception)
		}
		for _, binding := range bindings {
			key, err := cache.MetaNamespaceKeyFunc(binding)
			if err != nil {
				return err
			}
			policyNameToLabel[key] = reportutils.ValidatingAdmissionPolicyBindingLabel(binding)
		}
		for _, result := range observed.GetResults() {
			// if the policy did not change, keep the result
			label := policyNameToLabel[result.Policy]
			exceptionLabel := policyNameToLabel[result.Properties["exception"]]
			vapBindingLabel := policyNameToLabel[result.Properties["binding"]]
			if (label != "" && expected[label] == actual[label]) ||
				(exceptionLabel != "" && expected[exceptionLabel] == actual[exceptionLabel]) ||
				(vapBindingLabel != "" && expected[vapBindingLabel] == actual[vapBindingLabel]) {
				ruleResults = append(ruleResults, result)
			}
		}
	}
	// calculate necessary results
	for _, policy := range policies {
		reevaluate := false
		if policy.GetType() == engineapi.KyvernoPolicyType {
			for _, polex := range exceptions {
				if actual[reportutils.PolicyExceptionLabel(polex)] != polex.GetResourceVersion() {
					reevaluate = true
					break
				}
			}
		} else {
			for _, binding := range bindings {
				if actual[reportutils.ValidatingAdmissionPolicyBindingLabel(binding)] != binding.GetResourceVersion() {
					reevaluate = true
					break
				}
			}
		}
		if full || reevaluate || actual[reportutils.PolicyLabel(policy)] != policy.GetResourceVersion() {
			scanner := utils.NewScanner(logger, c.engine, c.config, c.jp, c.client)
			for _, result := range scanner.ScanResource(ctx, *target, nsLabels, bindings, policy) {
				if result.Error != nil {
					return result.Error
				} else if result.EngineResponse != nil {
					ruleResults = append(ruleResults, reportutils.EngineResponseToReportResults(*result.EngineResponse)...)
					utils.GenerateEvents(logger, c.eventGen, c.config, *result.EngineResponse)
				}
			}
		}
	}
	desired := reportutils.DeepCopy(observed)
	for key := range desired.GetLabels() {
		if reportutils.IsPolicyLabel(key) {
			delete(desired.GetLabels(), key)
		}
	}
	for _, policy := range policies {
		reportutils.SetPolicyLabel(desired, policy)
	}
	for _, exception := range exceptions {
		reportutils.SetPolicyExceptionLabel(desired, exception)
	}
	for _, binding := range bindings {
		reportutils.SetValidatingAdmissionPolicyBindingLabel(desired, binding)
	}
	reportutils.SetResourceVersionLabels(desired, target)
	reportutils.SetResults(desired, ruleResults...)
	if full || !controllerutils.HasAnnotation(desired, annotationLastScanTime) {
		controllerutils.SetAnnotation(desired, annotationLastScanTime, time.Now().Format(time.RFC3339))
	}
	if c.policyReports {
		return c.storeReport(ctx, observed, desired)
	}
	return nil
}

func (c *controller) storeReport(ctx context.Context, observed, desired kyvernov1alpha2.ReportInterface) error {
	var err error
	hasReport := observed.GetResourceVersion() != ""
	wantsReport := desired != nil && len(desired.GetResults()) != 0
	if !hasReport && !wantsReport {
		return nil
	} else if !hasReport && wantsReport {
		_, err = reportutils.CreateReport(ctx, desired, c.kyvernoClient)
		return err
	} else if hasReport && !wantsReport {
		if observed.GetNamespace() == "" {
			return c.kyvernoClient.ReportsV1().ClusterEphemeralReports().Delete(ctx, observed.GetName(), metav1.DeleteOptions{})
		} else {
			return c.kyvernoClient.ReportsV1().EphemeralReports(observed.GetNamespace()).Delete(ctx, observed.GetName(), metav1.DeleteOptions{})
		}
	} else {
		if utils.ReportsAreIdentical(observed, desired) {
			return nil
		}
		_, err = reportutils.UpdateReport(ctx, desired, c.kyvernoClient)
		return err
	}
}

func (c *controller) reconcile(ctx context.Context, log logr.Logger, key, namespace, name string) error {
	// try to find resource from the cache
	uid := types.UID(name)
	resource, gvk, exists := c.metadataCache.GetResourceHash(uid)
	// if the resource is not present it means we shouldn't have a report for it
	// we can delete the report, we will recreate one if the resource comes back
	if !exists {
		report, err := c.getMeta(namespace, name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		} else {
			if report.GetNamespace() == "" {
				return c.kyvernoClient.ReportsV1().ClusterEphemeralReports().Delete(ctx, report.GetName(), metav1.DeleteOptions{})
			} else {
				return c.kyvernoClient.ReportsV1().EphemeralReports(report.GetNamespace()).Delete(ctx, report.GetName(), metav1.DeleteOptions{})
			}
		}
	}
	// load all kyverno policies
	kyvernoPolicies, err := utils.FetchClusterPolicies(c.cpolLister)
	if err != nil {
		return err
	}
	if namespace != "" {
		pols, err := utils.FetchPolicies(c.polLister, namespace)
		if err != nil {
			return err
		}
		kyvernoPolicies = append(kyvernoPolicies, pols...)
	}
	// load background policies
	kyvernoPolicies = utils.RemoveNonBackgroundPolicies(kyvernoPolicies...)
	var policies []engineapi.GenericPolicy
	for _, pol := range kyvernoPolicies {
		policies = append(policies, engineapi.NewKyvernoPolicy(pol))
	}
	if c.vapLister != nil {
		// load validating admission policies
		vapPolicies, err := utils.FetchValidatingAdmissionPolicies(c.vapLister)
		if err != nil {
			return err
		}
		for _, pol := range vapPolicies {
			policies = append(policies, engineapi.NewValidatingAdmissionPolicy(pol))
		}
	}
	var vapBindings []admissionregistrationv1alpha1.ValidatingAdmissionPolicyBinding
	if c.vapBindingLister != nil {
		// load validating admission policy bindings
		vapBindings, err = utils.FetchValidatingAdmissionPolicyBindings(c.vapBindingLister)
		if err != nil {
			return err
		}
	}
	// load policy exceptions with background process enabled
	exceptions, err := utils.FetchPolicyExceptions(c.polexLister, namespace)
	if err != nil {
		return err
	}
	// we have the resource, check if we need to reconcile
	if needsReconcile, full, err := c.needsReconcile(namespace, name, resource.Hash, exceptions, vapBindings, policies...); err != nil {
		return err
	} else {
		defer func() {
			c.queue.AddAfter(key, c.forceDelay)
		}()
		if needsReconcile {
			return c.reconcileReport(ctx, namespace, name, full, uid, gvk, resource, exceptions, vapBindings, policies...)
		}
	}
	return nil
}
