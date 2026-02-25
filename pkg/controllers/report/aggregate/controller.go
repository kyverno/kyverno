package aggregate

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/controllers/report/utils"
	"github.com/kyverno/kyverno/pkg/openreports"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	openreportsclient "github.com/openreports/reports-api/pkg/client/clientset/versioned/typed/openreports.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	admissionregistrationv1informers "k8s.io/client-go/informers/admissionregistration/v1"
	admissionregistrationv1alpha1informers "k8s.io/client-go/informers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1informers "k8s.io/client-go/informers/admissionregistration/v1beta1"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Workers is the number of workers for this controller
	Workers        = 10
	ControllerName = "aggregate-report-controller"
	maxRetries     = 10
	enqueueDelay   = 10 * time.Second
	deletionGrace  = time.Minute * 2
)

type controller struct {
	// clients
	client   versioned.Interface
	orClient openreportsclient.OpenreportsV1alpha1Interface
	dclient  dclient.Interface

	// listers
	polLister      kyvernov1listers.PolicyLister
	cpolLister     kyvernov1listers.ClusterPolicyLister
	vpolLister     policiesv1beta1listers.ValidatingPolicyLister
	nvpolLister    policiesv1beta1listers.NamespacedValidatingPolicyLister
	ivpolLister    policiesv1beta1listers.ImageValidatingPolicyLister
	nivpolLister   policiesv1beta1listers.NamespacedImageValidatingPolicyLister
	gpolLister     policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister    policiesv1beta1listers.NamespacedGeneratingPolicyLister
	mpolLister     policiesv1beta1listers.MutatingPolicyLister
	nmpolLister    policiesv1beta1listers.NamespacedMutatingPolicyLister
	vapLister      admissionregistrationv1listers.ValidatingAdmissionPolicyLister
	mapLister      admissionregistrationv1beta1listers.MutatingAdmissionPolicyLister
	mapAlphaLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyLister
	ephrLister     cache.GenericLister
	cephrLister    cache.GenericLister

	// reportUUIDToPolicyCache maps report UUIDs to policies that affect them for targeted reconciliation.
	// This avoids processing all reports when a single policy changes.
	cacheMu                 *sync.Mutex
	reportUUIDToPolicyCache map[string]sets.Set[string]

	// queues
	frontQueue workqueue.TypedRateLimitingInterface[any]
	backQueue  workqueue.TypedRateLimitingInterface[any]
}

type policyMapEntry struct {
	policy kyvernov1.PolicyInterface
	rules  sets.Set[string]
}

func NewController(
	client versioned.Interface,
	orClient openreportsclient.OpenreportsV1alpha1Interface,
	dclient dclient.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	polInformer kyvernov1informers.PolicyInformer,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	vpolInformer policiesv1beta1informers.ValidatingPolicyInformer,
	nvpolInformer policiesv1beta1informers.NamespacedValidatingPolicyInformer,
	ivpolInformer policiesv1beta1informers.ImageValidatingPolicyInformer,
	nivpolInformer policiesv1beta1informers.NamespacedImageValidatingPolicyInformer,
	gpolInformer policiesv1beta1informers.GeneratingPolicyInformer,
	ngpolInformer policiesv1beta1informers.NamespacedGeneratingPolicyInformer,
	mpolInformer policiesv1beta1informers.MutatingPolicyInformer,
	nmpolInformer policiesv1beta1informers.NamespacedMutatingPolicyInformer,
	vapInformer admissionregistrationv1informers.ValidatingAdmissionPolicyInformer,
	mapInformer admissionregistrationv1beta1informers.MutatingAdmissionPolicyInformer,
	mapAlphaInformer admissionregistrationv1alpha1informers.MutatingAdmissionPolicyInformer,
) controllers.Controller {
	ephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))

	var (
		polrInformer  informers.GenericInformer
		cpolrInformer informers.GenericInformer
	)

	if orClient != nil {
		polrInformer = metadataFactory.ForResource(openreportsv1alpha1.SchemeGroupVersion.WithResource("reports"))
		cpolrInformer = metadataFactory.ForResource(openreportsv1alpha1.SchemeGroupVersion.WithResource("clusterreports"))
	} else {
		polrInformer = metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("policyreports"))
		cpolrInformer = metadataFactory.ForResource(policyreportv1alpha2.SchemeGroupVersion.WithResource("clusterpolicyreports"))
	}

	cacheMu := &sync.Mutex{}
	// Initialize cache for mapping report UUIDs to their affecting policies.
	reportUUIDToPolicyCache := make(map[string]sets.Set[string])
	selector := labels.SelectorFromSet(labels.Set{
		kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
	})

	// Background cleanup goroutine removes cache entries for deleted reports every 10 seconds.
	go func() {
		for {
			time.Sleep(time.Second * 10)
			// List all existing reports to identify which ones still exist
			reports, err := polrInformer.Lister().List(selector)
			if err != nil {
				logger.Error(err, "failed to list reports to clear the policy cache")
				continue
			}
			clusterReports, err := cpolrInformer.Lister().List(selector)
			if err != nil {
				logger.Error(err, "failed to list cluster reports to clear the policy cache")
				continue
			}
			// Build set of existing report UUIDs
			reportsMap := make(map[string]struct{})
			for _, r := range reports {
				reportMeta := r.(*metav1.PartialObjectMetadata)
				reportsMap[string(reportMeta.GetUID())] = struct{}{}
			}
			for _, cr := range clusterReports {
				reportMeta := cr.(*metav1.PartialObjectMetadata)
				reportsMap[string(reportMeta.GetUID())] = struct{}{}
			}
			// Remove cache entries for deleted reports
			cacheMu.Lock()
			for reportUID := range reportUUIDToPolicyCache {
				if _, ok := reportsMap[reportUID]; !ok {
					delete(reportUUIDToPolicyCache, reportUID)
				}
			}
			cacheMu.Unlock()
		}
	}()

	c := controller{
		client:                  client,
		dclient:                 dclient,
		orClient:                orClient,
		polLister:               polInformer.Lister(),
		cpolLister:              cpolInformer.Lister(),
		ephrLister:              ephrInformer.Lister(),
		cephrLister:             cephrInformer.Lister(),
		cacheMu:                 cacheMu,
		reportUUIDToPolicyCache: reportUUIDToPolicyCache,
		frontQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
		),
		backQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: ControllerName},
		),
	}
	if _, _, err := controllerutils.AddDelayedDefaultEventHandlers(logger, ephrInformer.Informer(), c.frontQueue, enqueueDelay); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, _, err := controllerutils.AddDelayedDefaultEventHandlers(logger, cephrInformer.Informer(), c.frontQueue, enqueueDelay); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	enqueueAll := func(items []runtime.Object) {
		for _, item := range items {
			itemMeta := item.(*metav1.PartialObjectMetadata)
			c.backQueue.AddAfter(controllerutils.MetaObjectToName(itemMeta), enqueueDelay)
		}
	}
	// enqueueReportsForPolicy queues only reports affected by a specific policy change using the cache.
	enqueueReportsForPolicy := func(o metav1.Object) {
		if list, err := polrInformer.Lister().List(selector); err == nil {
			// the cache has not been built yet, enqueue all reports for reconciliation
			cacheMu.Lock()
			cacheLen := len(reportUUIDToPolicyCache)
			cacheMu.Unlock()

			if cacheLen == 0 {
				enqueueAll(list)
				return
			}

			for _, item := range list {
				itemMeta := item.(*metav1.PartialObjectMetadata)
				cacheMu.Lock()
				policiesForReport, ok := reportUUIDToPolicyCache[string(itemMeta.GetUID())]
				cacheMu.Unlock()
				// the report doesn't exist in the cache, queue it for processing to eventually build the cache
				if !ok {
					c.backQueue.AddAfter(controllerutils.MetaObjectToName(itemMeta), enqueueDelay)
					continue
				}
				for _, polNsName := range sets.List(policiesForReport) {
					policyNameParts := strings.Split(polNsName, "/")
					if o.GetNamespace() == "" { // if its a cluster policy the cache will contain the policy name only
						if o.GetName() != policyNameParts[0] {
							continue
						}
					} else {
						if o.GetNamespace() != policyNameParts[0] {
							continue
						}
						if o.GetName() != policyNameParts[1] {
							continue
						}
					}
					c.backQueue.AddAfter(controllerutils.MetaObjectToName(itemMeta), enqueueDelay)
				}
			}
		}
		if list, err := cpolrInformer.Lister().List(selector); err == nil {
			// the cache has not been built yet, enqueue all reports for reconciliation
			cacheMu.Lock()
			cacheLen := len(reportUUIDToPolicyCache)
			cacheMu.Unlock()

			if cacheLen == 0 {
				enqueueAll(list)
				return
			}

			for _, item := range list {
				itemMeta := item.(*metav1.PartialObjectMetadata)
				cacheMu.Lock()
				policiesForReport, ok := reportUUIDToPolicyCache[string(itemMeta.GetUID())]
				cacheMu.Unlock()
				// the report doesn't exist in the cache, queue it for processing to eventually build the cache
				if !ok {
					c.backQueue.AddAfter(controllerutils.MetaObjectToName(itemMeta), enqueueDelay)
					continue
				}
				for _, polName := range sets.List(policiesForReport) {
					if o.GetName() != polName {
						continue
					}
					c.backQueue.AddAfter(controllerutils.MetaObjectToName(itemMeta), enqueueDelay)
				}
			}
		}
	}
	if _, err := controllerutils.AddEventHandlersT(
		polInformer.Informer(),
		func(o metav1.Object) { enqueueReportsForPolicy(o) },
		func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
		func(o metav1.Object) { enqueueReportsForPolicy(o) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if _, err := controllerutils.AddEventHandlersT(
		cpolInformer.Informer(),
		func(o metav1.Object) { enqueueReportsForPolicy(o) },
		func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
		func(o metav1.Object) { enqueueReportsForPolicy(o) },
	); err != nil {
		logger.Error(err, "failed to register event handlers")
	}
	if vpolInformer != nil {
		c.vpolLister = vpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			vpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if nvpolInformer != nil {
		c.nvpolLister = nvpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			nvpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if ivpolInformer != nil {
		c.ivpolLister = ivpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			ivpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if nivpolInformer != nil {
		c.nivpolLister = nivpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			nivpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mpolInformer != nil {
		c.mpolLister = mpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			mpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if nmpolInformer != nil {
		c.nmpolLister = nmpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			nmpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if gpolInformer != nil {
		c.gpolLister = gpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			gpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if ngpolInformer != nil {
		c.ngpolLister = ngpolInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			ngpolInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if vapInformer != nil {
		c.vapLister = vapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			vapInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mapInformer != nil {
		c.mapLister = mapInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			mapInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	if mapAlphaInformer != nil {
		c.mapAlphaLister = mapAlphaInformer.Lister()
		if _, err := controllerutils.AddEventHandlersT(
			mapAlphaInformer.Informer(),
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
			func(_, o metav1.Object) { enqueueReportsForPolicy(o) },
			func(o metav1.Object) { enqueueReportsForPolicy(o) },
		); err != nil {
			logger.Error(err, "failed to register event handlers")
		}
	}
	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	var group wait.Group
	group.StartWithContext(ctx, func(ctx context.Context) {
		controllerutils.Run(ctx, logger, ControllerName, time.Second, c.frontQueue, workers, maxRetries, c.frontReconcile)
	})
	group.StartWithContext(ctx, func(ctx context.Context) {
		controllerutils.Run(ctx, logger, ControllerName, time.Second, c.backQueue, workers, maxRetries, c.backReconcile)
	})
	group.Wait()
}

func (c *controller) createPolicyMap() (map[string]policyMapEntry, error) {
	results := map[string]policyMapEntry{}
	cpols, err := utils.FetchClusterPolicies(c.cpolLister)
	if err != nil {
		return nil, err
	}
	for _, cpol := range cpols {
		key := cache.MetaObjectToName(cpol).String()
		results[key] = policyMapEntry{
			policy: cpol,
			rules:  sets.New[string](),
		}
		for _, rule := range autogen.Default.ComputeRules(cpol, "") {
			results[key].rules.Insert(rule.Name)
		}
	}
	pols, err := utils.FetchPolicies(c.polLister, metav1.NamespaceAll)
	if err != nil {
		return nil, err
	}
	for _, pol := range pols {
		key := cache.MetaObjectToName(pol).String()
		results[key] = policyMapEntry{
			policy: pol,
			rules:  sets.New[string](),
		}
		for _, rule := range autogen.Default.ComputeRules(pol, "") {
			results[key].rules.Insert(rule.Name)
		}
	}
	return results, nil
}

func (c *controller) createVapMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.vapLister != nil {
		vaps, err := utils.FetchValidatingAdmissionPolicies(c.vapLister)
		if err != nil {
			return nil, err
		}
		for _, vap := range vaps {
			results.Insert(cache.MetaObjectToName(&vap).String())
		}
	}
	return results, nil
}

func (c *controller) createMappolMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.mapLister != nil {
		maps, err := utils.FetchMutatingAdmissionPolicies(c.mapLister)
		if err != nil {
			return nil, err
		}
		for _, pol := range maps {
			results.Insert(cache.MetaObjectToName(&pol).String())
		}
	}
	if c.mapAlphaLister != nil {
		maps, err := utils.FetchMutatingAdmissionPoliciesAlpha(c.mapAlphaLister)
		if err != nil {
			return nil, err
		}
		for _, pol := range maps {
			results.Insert(cache.MetaObjectToName(&pol).String())
		}
	}
	return results, nil
}

func (c *controller) createVPolMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.vpolLister != nil {
		vpols, err := utils.FetchValidatingPolicies(c.vpolLister)
		if err != nil {
			return nil, err
		}
		for _, vpol := range vpols {
			results.Insert(cache.MetaObjectToName(&vpol).String())
		}
	}
	if c.nvpolLister != nil {
		vpols, err := utils.FetchNamespacedValidatingPolicies(c.nvpolLister, "")
		if err != nil {
			return nil, err
		}
		for _, vpol := range vpols {
			results.Insert(cache.MetaObjectToName(&vpol).String())
		}
	}
	return results, nil
}

func (c *controller) createIVPolMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.ivpolLister != nil {
		ivpols, err := utils.FetchImageVerificationPolicies(c.ivpolLister)
		if err != nil {
			return nil, err
		}
		for _, ivpol := range ivpols {
			results.Insert(cache.MetaObjectToName(&ivpol).String())
		}
	}
	if c.nivpolLister != nil {
		ivpols, err := utils.FetchNamespacedImageVerificationPolicies(c.nivpolLister, "")
		if err != nil {
			return nil, err
		}
		for _, ivpol := range ivpols {
			results.Insert(cache.MetaObjectToName(&ivpol).String())
		}
	}
	return results, nil
}

func (c *controller) createGPOLMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.gpolLister != nil {
		gpols, err := utils.FetchGeneratingPolicy(c.gpolLister)
		if err != nil {
			return nil, err
		}
		for _, gpol := range gpols {
			results.Insert(cache.MetaObjectToName(&gpol).String())
		}
	}
	if c.ngpolLister != nil {
		gpols, err := utils.FetchNamespacedGeneratingPolicies(c.ngpolLister, "")
		if err != nil {
			return nil, err
		}
		for _, gpol := range gpols {
			results.Insert(cache.MetaObjectToName(&gpol).String())
		}
	}
	return results, nil
}

func (c *controller) createMPOLMap() (sets.Set[string], error) {
	results := sets.New[string]()
	if c.mpolLister != nil {
		mpols, err := utils.FetchMutatingPolicies(c.mpolLister)
		if err != nil {
			return nil, err
		}
		for _, mpol := range mpols {
			results.Insert(cache.MetaObjectToName(&mpol).String())
		}
	}
	if c.nmpolLister != nil {
		mpols, err := utils.FetchNamespacedMutatingPolicies(c.nmpolLister, "")
		if err != nil {
			return nil, err
		}
		for _, mpol := range mpols {
			results.Insert(cache.MetaObjectToName(&mpol).String())
		}
	}
	return results, nil
}

func (c *controller) findOwnedEphemeralReports(ctx context.Context, namespace, name string) ([]reportsv1.ReportInterface, error) {
	selector, err := reportutils.SelectorResourceUidEquals(types.UID(name))
	if err != nil {
		return nil, err
	}
	var results []reportsv1.ReportInterface
	if namespace == "" {
		reports, err := c.client.ReportsV1().ClusterEphemeralReports().List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, report := range reports.Items {
			if len(report.OwnerReferences) != 0 {
				report := report
				results = append(results, &report)
			}
		}
	} else {
		reports, err := c.client.ReportsV1().EphemeralReports(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, report := range reports.Items {
			if len(report.OwnerReferences) != 0 {
				report := report
				results = append(results, &report)
			}
		}
	}
	return results, nil
}

func (c *controller) getReport(ctx context.Context, namespace, name string) (reportsv1.ReportInterface, error) {
	if c.orClient == nil {
		if namespace == "" {
			report, err := c.client.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}
			return openreports.NewWGCpolAdapter(report), nil
		} else {
			report, err := c.client.Wgpolicyk8sV1alpha2().PolicyReports(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}
			return openreports.NewWGPolAdapter(report), nil
		}
	}
	// openreports client wasn't nil, fetch an openreports report
	if namespace == "" {
		report, err := c.orClient.ClusterReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return &openreports.ClusterReportAdapter{ClusterReport: report}, nil
	} else {
		report, err := c.orClient.Reports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return &openreports.ReportAdapter{Report: report}, nil
	}
}

func (c *controller) lookupEphemeralReportMeta(_ context.Context, namespace, name string) (*metav1.PartialObjectMetadata, error) {
	if namespace == "" {
		obj, err := c.cephrLister.Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(*metav1.PartialObjectMetadata), nil
	} else {
		obj, err := c.ephrLister.ByNamespace(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return obj.(*metav1.PartialObjectMetadata), nil
	}
}

func (c *controller) getEphemeralReport(ctx context.Context, namespace, name string) (reportsv1.ReportInterface, error) {
	if namespace == "" {
		obj, err := c.client.ReportsV1().ClusterEphemeralReports().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, err
	} else {
		obj, err := c.client.ReportsV1().EphemeralReports(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return obj, err
	}
}

func (c *controller) deleteEphemeralReport(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		return c.client.ReportsV1().ClusterEphemeralReports().Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		return c.client.ReportsV1().EphemeralReports(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
}

func (c *controller) findResource(ctx context.Context, reportMeta *metav1.PartialObjectMetadata) (*unstructured.Unstructured, error) {
	gvr := reportutils.GetResourceGVR(reportMeta)
	dyn := c.dclient.GetDynamicInterface().Resource(gvr)
	namespace, name := reportutils.GetResourceNamespaceAndName(reportMeta)
	var iface dynamic.ResourceInterface
	if namespace == "" {
		iface = dyn
	} else {
		iface = dyn.Namespace(namespace)
	}
	resource, err := iface.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	return resource, nil
}

func (c *controller) adopt(ctx context.Context, reportMeta *metav1.PartialObjectMetadata) (bool, bool) {
	resource, err := c.findResource(ctx, reportMeta)
	if err != nil {
		if apierrors.IsForbidden(err) {
			return false, true
		}
		return false, false
	}
	if resource == nil {
		return false, false
	}
	report, err := c.getEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	if err != nil {
		return false, false
	}
	if report == nil {
		return false, false
	}
	controllerutils.SetOwner(report, resource.GetAPIVersion(), resource.GetKind(), resource.GetName(), resource.GetUID())
	reportutils.SetResourceUid(report, resource.GetUID())
	if _, err := updateReport(ctx, report, c.client, c.orClient); err != nil {
		return false, false
	}
	return true, false
}

func (c *controller) frontReconcile(ctx context.Context, logger logr.Logger, _, namespace, name string) error {
	reportMeta, err := c.lookupEphemeralReportMeta(ctx, namespace, name)
	// try to lookup metadata from lister
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	// check if it is owned already
	if len(reportMeta.OwnerReferences) != 0 {
		defer func() {
			obj := cache.ObjectName{Namespace: namespace, Name: string(reportMeta.OwnerReferences[0].UID)}
			c.backQueue.Add(obj.String())
		}()
		return nil
	}
	// try to find the owner
	if adopted, forbidden := c.adopt(ctx, reportMeta); adopted {
		return nil
	} else if forbidden {
		logger.V(3).Info("deleting because insufficient permission to fetch resource")
		return c.deleteEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	}
	// if not found and too old, forget about it
	if isTooOld(reportMeta) {
		return c.deleteEphemeralReport(ctx, reportMeta.GetNamespace(), reportMeta.GetName())
	}
	// else try again later
	c.frontQueue.AddAfter(controllerutils.MetaObjectToName(reportMeta), enqueueDelay)
	return nil
}

func (c *controller) backReconcile(ctx context.Context, logger logr.Logger, _, namespace, name string) (err error) {
	var reports []reportsv1.ReportInterface
	// get the report
	// if we don't have a report, we will eventually create one
	report, err := c.getReport(ctx, namespace, name)
	if err != nil {
		return err
	}
	if report != nil {
		reports = append(reports, report)
	}
	// get ephemeral reports
	ephemeralReports, err := c.findOwnedEphemeralReports(ctx, namespace, name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	// if there was no error aggregating the report we can delete ephemeral reports
	defer func() {
		if err == nil {
			for _, ephemeralReport := range ephemeralReports {
				if err := deleteReport(ctx, ephemeralReport, c.client, c.orClient); err != nil {
					logger.Error(err, "failed to delete ephemeral report")
				}
			}
		}
	}()
	// aggregate reports
	policyMap, err := c.createPolicyMap()
	if err != nil {
		return err
	}
	vapMap, err := c.createVapMap()
	if err != nil {
		return err
	}
	mappolMap, err := c.createMappolMap()
	if err != nil {
		return err
	}
	vpolMap, err := c.createVPolMap()
	if err != nil {
		return err
	}
	ivpolMap, err := c.createIVPolMap()
	if err != nil {
		return err
	}
	gpolMap, err := c.createGPOLMap()
	if err != nil {
		return err
	}
	mpolMap, err := c.createMPOLMap()
	if err != nil {
		return err
	}
	maps := maps{
		pol:    policyMap,
		vap:    vapMap,
		mappol: mappolMap,
		vpol:   vpolMap,
		ivpol:  ivpolMap,
		gpol:   gpolMap,
		mpol:   mpolMap,
	}
	reports = append(reports, ephemeralReports...)
	merged := map[string]openreportsv1alpha1.ReportResult{}
	mergeReports(maps, merged, types.UID(name), reports...)
	policySet := sets.New[string]() // collect policies for cache population
	results := make([]openreportsv1alpha1.ReportResult, 0, len(merged))
	for _, result := range merged {
		results = append(results, result)
		if result.Policy != "" {
			policySet.Insert(result.Policy)
		}
	}

	if len(results) == 0 {
		if report != nil {
			return deleteReport(ctx, report, c.client, c.orClient)
		}
	} else {
		if report == nil {
			owner := ephemeralReports[0].GetOwnerReferences()[0]
			scope := &corev1.ObjectReference{
				Kind:       owner.Kind,
				Namespace:  namespace,
				Name:       owner.Name,
				UID:        owner.UID,
				APIVersion: owner.APIVersion,
			}
			report = reportutils.NewPolicyReport(namespace, name, scope, c.orClient != nil)
			controllerutils.SetOwner(report, owner.APIVersion, owner.Kind, owner.Name, owner.UID)
		}
		reportutils.SetResults(report, results...)
		if report.GetResourceVersion() == "" {
			r, err := reportutils.CreatePermanentReport(ctx, report, c.client, c.orClient)
			if err != nil {
				return err
			}

			uuid := r.GetUID()
			c.cacheMu.Lock()
			c.reportUUIDToPolicyCache[string(uuid)] = policySet
			c.cacheMu.Unlock()
		} else {
			r, err := updateReport(ctx, report, c.client, c.orClient)
			if err != nil {
				return err
			}
			uuid := r.GetUID()
			c.cacheMu.Lock()
			c.reportUUIDToPolicyCache[string(uuid)] = policySet
			c.cacheMu.Unlock()
		}
	}
	return nil
}
