package background

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	celpolicies "github.com/kyverno/kyverno/pkg/cel/policies"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	policiesv1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1alpha1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
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
	vpolLister       policiesv1alpha1listers.ValidatingPolicyLister
	mpolLister       policiesv1alpha1listers.MutatingPolicyLister
	ivpolLister      policiesv1alpha1listers.ImageValidatingPolicyLister
	polexLister      kyvernov2listers.PolicyExceptionLister
	celpolexListener policiesv1alpha1listers.PolicyExceptionLister
	vapLister        admissionregistrationv1listers.ValidatingAdmissionPolicyLister
	vapBindingLister admissionregistrationv1listers.ValidatingAdmissionPolicyBindingLister
	mapLister        admissionregistrationv1alpha1listers.MutatingAdmissionPolicyLister
	mapBindingLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyBindingLister
	bgscanrLister    cache.GenericLister
	cbgscanrLister   cache.GenericLister
	nsLister         corev1listers.NamespaceLister

	// queue
	queue workqueue.TypedRateLimitingInterface[string]

	// cache
	metadataCache resource.MetadataCache
	forceDelay    time.Duration

	// config
	config        config.Configuration
	jp            jmespath.Interface
	eventGen      event.Interface
	policyReports bool
	reportsConfig reportutils.ReportingConfiguration
	gctxStore     gctxstore.Store

	typeConverter patch.TypeConverterManager
}

func NewController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	engine engineapi.Engine,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vpolInformer policiesv1alpha1informers.ValidatingPolicyInformer,
	mpolInformer policiesv1alpha1informers.MutatingPolicyInformer,
	ivpolInformer policiesv1alpha1informers.ImageValidatingPolicyInformer,
	celpolexlInformer policiesv1alpha1informers.PolicyExceptionInformer,
	polexInformer kyvernov2informers.PolicyExceptionInformer,
	vapInformer admissionregistrationv1informers.ValidatingAdmissionPolicyInformer,
	vapBindingInformer admissionregistrationv1informers.ValidatingAdmissionPolicyBindingInformer,
	mapInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyInformer,
	mapBindingInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyBindingInformer,
	nsInformer corev1informers.NamespaceInformer,
	metadataCache resource.MetadataCache,
	forceDelay time.Duration,
	config config.Configuration,
	jp jmespath.Interface,
	eventGen event.Interface,
	policyReports bool,
	reportsConfig reportutils.ReportingConfiguration,
	gctxStore gctxstore.Store,
	typeConverter patch.TypeConverterManager,
) controllers.Controller {
	ephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{Name: ControllerName},
	)
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
		reportsConfig:  reportsConfig,
		gctxStore:      gctxStore,
		typeConverter:  typeConverter,
	}
	if vpolInformer != nil {
		c.vpolLister = vpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(vpolInformer.Informer(), c.addVP, c.updateVP, c.deleteVP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mpolInformer != nil {
		c.mpolLister = mpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(mpolInformer.Informer(), c.addMP, c.updateMP, c.deleteMP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if ivpolInformer != nil {
		c.ivpolLister = ivpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(ivpolInformer.Informer(), c.addIVP, c.updateIVP, c.deleteIVP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if celpolexlInformer != nil {
		c.celpolexListener = celpolexlInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(celpolexlInformer.Informer(), c.addCELException, c.updateCELException, c.deleteCELPolicy); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
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
	if mapInformer != nil {
		c.mapLister = mapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(mapInformer.Informer(), c.addMAP, c.updateMAP, c.deleteMAP); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mapBindingInformer != nil {
		c.mapBindingLister = mapBindingInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(mapBindingInformer.Informer(), c.addMAPBinding, c.updateMAPBinding, c.deleteMAPBinding); err != nil {
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
	logger.V(2).Info("background scan", "interval", c.forceDelay.Abs().String())
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

func (c *controller) addException(obj *kyvernov2.PolicyException) {
	c.enqueueResources()
}

func (c *controller) updateException(old, obj *kyvernov2.PolicyException) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteCELPolicy(obj *policiesv1alpha1.PolicyException) {
	c.enqueueResources()
}

func (c *controller) addCELException(obj *policiesv1alpha1.PolicyException) {
	c.enqueueResources()
}

func (c *controller) updateCELException(old, obj *policiesv1alpha1.PolicyException) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteException(obj *kyvernov2.PolicyException) {
	c.enqueueResources()
}

func (c *controller) addVP(obj *policiesv1alpha1.ValidatingPolicy) {
	c.enqueueResources()
}

func (c *controller) updateVP(old, obj *policiesv1alpha1.ValidatingPolicy) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteVP(obj *policiesv1alpha1.ValidatingPolicy) {
	c.enqueueResources()
}

func (c *controller) addMP(obj *policiesv1alpha1.MutatingPolicy) {
	c.enqueueResources()
}

func (c *controller) updateMP(old, obj *policiesv1alpha1.MutatingPolicy) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteMP(obj *policiesv1alpha1.MutatingPolicy) {
	c.enqueueResources()
}

func (c *controller) addIVP(obj *policiesv1alpha1.ImageValidatingPolicy) {
	c.enqueueResources()
}

func (c *controller) updateIVP(old, obj *policiesv1alpha1.ImageValidatingPolicy) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteIVP(obj *policiesv1alpha1.ImageValidatingPolicy) {
	c.enqueueResources()
}

func (c *controller) addVAP(obj *admissionregistrationv1.ValidatingAdmissionPolicy) {
	c.enqueueResources()
}

func (c *controller) updateVAP(old, obj *admissionregistrationv1.ValidatingAdmissionPolicy) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteVAP(obj *admissionregistrationv1.ValidatingAdmissionPolicy) {
	c.enqueueResources()
}

func (c *controller) addVAPBinding(obj *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	c.enqueueResources()
}

func (c *controller) updateVAPBinding(old, obj *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteVAPBinding(obj *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	c.enqueueResources()
}

func (c *controller) addMAP(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueResources()
}

func (c *controller) updateMAP(old, obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteMAP(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueResources()
}

func (c *controller) addMAPBinding(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueResources()
}

func (c *controller) updateMAPBinding(old, obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		c.enqueueResources()
	}
}

func (c *controller) deleteMAPBinding(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueResources()
}

func (c *controller) enqueueResources() {
	for _, key := range c.metadataCache.GetAllResourceKeys() {
		c.queue.Add(key)
	}
}

func (c *controller) getReport(ctx context.Context, namespace, name string) (reportsv1.ReportInterface, error) {
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

func (c *controller) needsReconcile(
	namespace string,
	name string,
	hash string,
	exceptions []kyvernov2.PolicyException,
	vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	mapBindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	policies ...engineapi.GenericPolicy,
) (bool, bool, error) {
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
	for _, binding := range vapBindings {
		expected[reportutils.ValidatingAdmissionPolicyBindingLabel(binding)] = binding.GetResourceVersion()
	}
	for _, binding := range mapBindings {
		expected[reportutils.MutatingAdmissionPolicyBindingLabel(binding)] = binding.GetResourceVersion()
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
	gvr schema.GroupVersionResource,
	resource resource.Resource,
	exceptions []kyvernov2.PolicyException,
	celexceptions []*policiesv1alpha1.PolicyException,
	vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding,
	mapBindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding,
	policies ...engineapi.GenericPolicy,
) error {
	// namespace labels to be used by the scanner
	var ns *corev1.Namespace
	if namespace != "" {
		namespace, err := c.nsLister.Get(namespace)
		if err != nil {
			return err
		}
		ns = namespace
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
	for _, binding := range vapBindings {
		expected[reportutils.ValidatingAdmissionPolicyBindingLabel(binding)] = binding.GetResourceVersion()
	}
	for _, binding := range mapBindings {
		expected[reportutils.MutatingAdmissionPolicyBindingLabel(binding)] = binding.GetResourceVersion()
	}
	actual := map[string]string{}
	for key, value := range observed.GetLabels() {
		if reportutils.IsPolicyLabel(key) {
			actual[key] = value
		}
	}
	var ruleResults []openreportsv1alpha1.ReportResult
	if !full {
		policyNameToLabel := map[string]string{}
		for _, policy := range policies {
			var key string
			if policy.AsKyvernoPolicy() != nil {
				key = cache.MetaObjectToName(policy.AsKyvernoPolicy()).String()
			} else if policy.AsValidatingAdmissionPolicy() != nil {
				key = cache.MetaObjectToName(policy.AsValidatingAdmissionPolicy().GetDefinition()).String()
			} else if policy.AsMutatingAdmissionPolicy() != nil {
				key = cache.MetaObjectToName(policy.AsMutatingAdmissionPolicy().GetDefinition()).String()
			} else if policy.AsValidatingPolicy() != nil {
				key = cache.MetaObjectToName(policy.AsValidatingPolicy()).String()
			} else if policy.AsImageValidatingPolicy() != nil {
				key = cache.MetaObjectToName(policy.AsImageValidatingPolicy()).String()
			} else if policy.AsMutatingPolicy() != nil {
				key = cache.MetaObjectToName(policy.AsMutatingPolicy()).String()
			}
			policyNameToLabel[key] = reportutils.PolicyLabel(policy)
		}
		for i, exception := range exceptions {
			key := cache.MetaObjectToName(&exceptions[i]).String()
			policyNameToLabel[key] = reportutils.PolicyExceptionLabel(exception)
		}
		for _, binding := range vapBindings {
			key := cache.MetaObjectToName(&binding).String()
			policyNameToLabel[key] = reportutils.ValidatingAdmissionPolicyBindingLabel(binding)
		}
		for _, binding := range mapBindings {
			key := cache.MetaObjectToName(&binding).String()
			policyNameToLabel[key] = reportutils.MutatingAdmissionPolicyBindingLabel(binding)
		}
		for _, result := range observed.GetResults() {
			// The result is kept as it is if:
			// 1. The Kyverno policy and its matched exceptions are unchanged
			// 2. The ValidatingAdmissionPolicy and its matched binding are unchanged
			keepResult := true
			exception := result.Properties["exceptions"]
			exceptions := strings.Split(exception, ",")
			for _, exception := range exceptions {
				exceptionLabel := policyNameToLabel[exception]
				if exceptionLabel != "" && expected[exceptionLabel] != actual[exceptionLabel] {
					keepResult = false
					break
				}
			}
			label := policyNameToLabel[result.Policy]
			vapBindingLabel := policyNameToLabel[result.Properties["binding"]]
			mapBindingLabel := policyNameToLabel[result.Properties["mapBinding"]]
			if (label != "" && expected[label] == actual[label]) ||
				(vapBindingLabel != "" && expected[vapBindingLabel] == actual[vapBindingLabel]) ||
				(mapBindingLabel != "" && expected[mapBindingLabel] == actual[mapBindingLabel]) || keepResult {
				ruleResults = append(ruleResults, result)
			}
		}
	}
	// calculate necessary results
	for _, policy := range policies {
		if vpol := policy.AsValidatingPolicy(); vpol != nil && vpol.Status.Generated {
			continue
		}
		if mpol := policy.AsMutatingPolicy(); mpol != nil && mpol.Status.Generated {
			continue
		}

		reevaluate := false
		if policy.AsKyvernoPolicy() != nil {
			for _, polex := range exceptions {
				if actual[reportutils.PolicyExceptionLabel(polex)] != polex.GetResourceVersion() {
					reevaluate = true
					break
				}
			}
		} else if policy.AsValidatingAdmissionPolicy() != nil {
			for _, binding := range vapBindings {
				if actual[reportutils.ValidatingAdmissionPolicyBindingLabel(binding)] != binding.GetResourceVersion() {
					reevaluate = true
					break
				}
			}
		} else if policy.AsMutatingAdmissionPolicy() != nil {
			for _, binding := range mapBindings {
				if actual[reportutils.MutatingAdmissionPolicyBindingLabel(binding)] != binding.GetResourceVersion() {
					reevaluate = true
					break
				}
			}
		}
		if full || reevaluate || actual[reportutils.PolicyLabel(policy)] != policy.GetResourceVersion() {
			scanner := utils.NewScanner(logger, c.engine, c.config, c.jp, c.client, c.reportsConfig, c.gctxStore, c.typeConverter)
			for _, result := range scanner.ScanResource(ctx, *target, gvr, "", ns, vapBindings, mapBindings, celexceptions, policy) {
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
	for _, binding := range vapBindings {
		reportutils.SetValidatingAdmissionPolicyBindingLabel(desired, binding)
	}
	for _, binding := range mapBindings {
		reportutils.SetMutatingAdmissionPolicyBindingLabel(desired, binding)
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

func (c *controller) storeReport(ctx context.Context, observed, desired reportsv1.ReportInterface) error {
	var err error
	hasReport := observed.GetResourceVersion() != ""
	wantsReport := desired != nil && len(desired.GetResults()) != 0
	if !hasReport && !wantsReport {
		return nil
	} else if !hasReport && wantsReport {
		err = breaker.GetReportsBreaker().Do(ctx, func(context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, desired, c.kyvernoClient)
			if err != nil {
				return err
			}
			return nil
		})
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
		_, err = reportutils.UpdateReport(ctx, desired, c.kyvernoClient, nil)
		return err
	}
}

func (c *controller) reconcile(ctx context.Context, log logr.Logger, key, namespace, name string) error {
	// try to find resource from the cache
	uid := types.UID(name)
	resource, gvk, gvr, exists := c.metadataCache.GetResourceHash(uid)
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

	kyvernoPolicies = utils.RemoveNonBackgroundPolicies(kyvernoPolicies...)
	policies := make([]engineapi.GenericPolicy, 0, len(kyvernoPolicies))
	for _, pol := range kyvernoPolicies {
		policies = append(policies, engineapi.NewKyvernoPolicy(pol))
	}
	if c.vpolLister != nil {
		vpols, err := utils.FetchValidatingPolicies(c.vpolLister)
		if err != nil {
			return err
		}
		for _, vpol := range celpolicies.RemoveNoneBackgroundPolicies(vpols) {
			policies = append(policies, engineapi.NewValidatingPolicy(&vpol))
		}
	}
	if c.mpolLister != nil {
		mpols, err := utils.FetchMutatingPolicies(c.mpolLister)
		if err != nil {
			return err
		}
		for _, mpol := range celpolicies.RemoveNoneBackgroundPolicies(mpols) {
			policies = append(policies, engineapi.NewMutatingPolicy(&mpol))
		}
	}
	if c.ivpolLister != nil {
		ivpols, err := utils.FetchImageVerificationPolicies(c.ivpolLister)
		if err != nil {
			return err
		}
		for _, vpol := range celpolicies.RemoveNoneBackgroundPolicies(ivpols) {
			policies = append(policies, engineapi.NewImageValidatingPolicy(&vpol))
		}
	}
	if c.vapLister != nil {
		vapPolicies, err := utils.FetchValidatingAdmissionPolicies(c.vapLister)
		if err != nil {
			return err
		}
		for _, pol := range vapPolicies {
			policies = append(policies, engineapi.NewValidatingAdmissionPolicy(&pol))
		}
	}
	var vapBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	if c.vapBindingLister != nil {
		// load validating admission policy bindings
		vapBindings, err = utils.FetchValidatingAdmissionPolicyBindings(c.vapBindingLister)
		if err != nil {
			return err
		}
	}
	if c.mapLister != nil {
		// load mutating admission policies
		mapPolicies, err := utils.FetchMutatingAdmissionPolicies(c.mapLister)
		if err != nil {
			return err
		}
		for _, pol := range mapPolicies {
			policies = append(policies, engineapi.NewMutatingAdmissionPolicy(&pol))
		}
	}
	var mapBindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
	if c.mapBindingLister != nil {
		// load mutating admission policy bindings
		mapBindings, err = utils.FetchMutatingAdmissionPolicyBindings(c.mapBindingLister)
		if err != nil {
			return err
		}
	}
	// load policy exceptions with background process enabled
	exceptions, err := utils.FetchPolicyExceptions(c.polexLister, namespace)
	if err != nil {
		return err
	}
	// load celexceptions with background process enabled
	celexceptions, err := utils.FetchCELPolicyExceptions(c.celpolexListener, namespace)
	if err != nil {
		return err
	}
	// we have the resource, check if we need to reconcile
	if needsReconcile, full, err := c.needsReconcile(namespace, name, resource.Hash, exceptions, vapBindings, mapBindings, policies...); err != nil {
		return err
	} else {
		defer func() {
			c.queue.AddAfter(key, c.forceDelay)
		}()
		if needsReconcile {
			return c.reconcileReport(ctx, namespace, name, full, uid, gvk, gvr, resource, exceptions, celexceptions, vapBindings, mapBindings, policies...)
		}
	}
	return nil
}
